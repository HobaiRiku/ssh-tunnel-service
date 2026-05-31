<script setup lang="ts">
import { computed, markRaw, nextTick, ref, watch } from 'vue'
import { MarkerType, PanOnScrollMode, VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import type { Edge, Node } from '@vue-flow/core'
import type { Remote, TunnelStatus } from '@/api/client'
import TunnelEdge from '@/components/edges/TunnelEdge.vue'
import RemoteGroupNode from '@/components/nodes/RemoteGroupNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'
import { topologyViewState } from '@/components/topologyViewState'
import { useI18n } from '@/i18n'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

const props = defineProps<{
  tunnels: TunnelStatus[]
  remotes: Remote[]
  loading: boolean
  selectedTunnelId?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', tunnel: TunnelStatus): void
}>()

const { viewport, setViewport, dimensions, onInit, onMove } = useVueFlow()
const { t } = useI18n()

// View state persisted across remounts (table <-> topology toggle, route changes)
// so returning to the topology keeps the user's zoom / scroll / remote tab.
const persisted = topologyViewState

const nodeTypes = {
  remoteGroup: markRaw(RemoteGroupNode),
  tunnel: markRaw(TunnelNode),
  target: markRaw(TargetNode),
}
const edgeTypes = {
  tunnel: markRaw(TunnelEdge),
}

const tunnelMap = computed(() => new Map(props.tunnels.map((tunnel) => [tunnel.id, tunnel] as const)))

const GROUP_HEADER_H = 60
const TUNNEL_SLOT_H = 128
const GROUP_PAD_TOP = 18
const GROUP_PAD_BOT = 22
const GROUP_W = 320
const TUNNEL_W = 272
const TUNNEL_INDENT = (GROUP_W - TUNNEL_W) / 2
const TARGET_X = 620
const GROUP_GAP = 40

// Horizontal span of the laid-out content; used to keep the canvas centered.
const CONTENT_W = 800
const MIN_ZOOM = 0.55
const MAX_ZOOM = 1.6
const PAD_X = 40
const PAD_Y = 36

type Endpoint = {
  host: string
  port: number
}

function getRemoteEndpoint(tunnel: TunnelStatus): Endpoint {
  return tunnel.direction === '-L'
    ? { host: tunnel.target_host, port: tunnel.target_port }
    : { host: tunnel.bind_address, port: tunnel.bind_port }
}

function getLocalEndpoint(tunnel: TunnelStatus): Endpoint {
  return tunnel.direction === '-L'
    ? { host: tunnel.bind_address, port: tunnel.bind_port }
    : { host: tunnel.target_host, port: tunnel.target_port }
}

const isEmpty = computed(() => !props.loading && props.remotes.length === 0)
const hasTunnels = computed(() => props.tunnels.length > 0)

// ---- Remote tab filtering ----------------------------------------------------
const activeRemoteId = ref<string | null>(
  persisted.remoteId !== undefined ? persisted.remoteId : null,
)

const filteredRemotes = computed(() => {
  if (!activeRemoteId.value) return props.remotes
  const found = props.remotes.filter((remote) => remote.id === activeRemoteId.value)
  return found.length > 0 ? found : props.remotes
})

const visibleRemoteIds = computed(() => new Set(filteredRemotes.value.map((remote) => remote.id)))
const visibleTunnels = computed(() => props.tunnels.filter((tunnel) => visibleRemoteIds.value.has(tunnel.remote_id)))

function selectRemote(id: string | null) {
  if (activeRemoteId.value === id) return
  activeRemoteId.value = id
  persisted.remoteId = id
  // New tab => start from the top of the (re-laid-out) content.
  void nextTick(() => apply({ y: PAD_Y, animate: true }))
}

// Drop a stale tab if the remote it points at was removed.
watch(
  () => props.remotes,
  (remotes) => {
    if (activeRemoteId.value && !remotes.some((remote) => remote.id === activeRemoteId.value)) {
      activeRemoteId.value = null
      persisted.remoteId = null
    }
  },
)

// ---- Node / edge layout ------------------------------------------------------
const layout = computed<{ nodes: Node[]; height: number }>(() => {
  const nodes: Node[] = []
  let yOffset = 0

  for (const remote of filteredRemotes.value) {
    const remoteTunnels = props.tunnels.filter((tunnel) => tunnel.remote_id === remote.id)
    const slotCount = Math.max(remoteTunnels.length, 1)
    const groupH = GROUP_HEADER_H + GROUP_PAD_TOP + slotCount * TUNNEL_SLOT_H + GROUP_PAD_BOT

    nodes.push({
      id: `group-${remote.id}`,
      type: 'remoteGroup',
      position: { x: 0, y: yOffset },
      data: { label: remote.name, host: remote.host, port: remote.port, user: remote.user },
      style: { width: `${GROUP_W}px`, height: `${groupH}px`, overflow: 'visible' },
      draggable: false,
      selectable: false,
      zIndex: 0,
    })

    remoteTunnels.forEach((tunnel, index) => {
      const childY = GROUP_HEADER_H + GROUP_PAD_TOP + index * TUNNEL_SLOT_H
      const absY = yOffset + childY
      const onSelect = () => emit('select', tunnel)
      const remoteEndpoint = getRemoteEndpoint(tunnel)
      const localEndpoint = getLocalEndpoint(tunnel)

      nodes.push({
        id: `tunnel-${tunnel.id}`,
        type: 'tunnel',
        parentNode: `group-${remote.id}`,
        extent: 'parent',
        position: { x: TUNNEL_INDENT, y: childY },
        data: {
          label: tunnel.name || tunnel.id,
          direction: tunnel.direction,
          remoteHost: remoteEndpoint.host,
          remotePort: remoteEndpoint.port,
          localHost: localEndpoint.host,
          localPort: localEndpoint.port,
          state: tunnel.state,
          selected: tunnel.id === props.selectedTunnelId,
          onSelect,
        },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })

      nodes.push({
        id: `target-${tunnel.id}`,
        type: 'target',
        position: { x: TARGET_X, y: absY + 38 },
        data: {
          host: localEndpoint.host,
          port: localEndpoint.port,
          selected: tunnel.id === props.selectedTunnelId,
          onSelect,
        },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })
    })

    yOffset += groupH + GROUP_GAP
  }

  // Trailing GROUP_GAP isn't real content; trim it so centering math is honest.
  const height = yOffset > 0 ? yOffset - GROUP_GAP : 0
  return { nodes, height }
})

const flowNodes = computed<Node[]>(() => layout.value.nodes)
const contentHeight = computed(() => layout.value.height)

const flowEdges = computed<Edge[]>(() => {
  return visibleTunnels.value.map((tunnel) => {
    const remoteEndpoint = getRemoteEndpoint(tunnel)
    const localEndpoint = getLocalEndpoint(tunnel)
    const stroke = tunnel.id === props.selectedTunnelId
      ? '#2563eb'
      : tunnel.state === 'running'
        ? '#22c55e'
        : tunnel.state === 'error'
          ? '#ef4444'
          : '#94a3b8'

    return {
      id: `edge-${tunnel.id}`,
      source: `tunnel-${tunnel.id}`,
      target: `target-${tunnel.id}`,
      type: 'tunnel',
      animated: tunnel.state === 'running',
      data: {
        remotePort: remoteEndpoint.port,
        localPort: localEndpoint.port,
        selected: tunnel.id === props.selectedTunnelId,
      },
      markerEnd: { type: MarkerType.ArrowClosed, width: 22, height: 22, strokeWidth: 1, color: stroke, markerUnits: 'userSpaceOnUse' },
      style: {
        stroke,
        strokeWidth: tunnel.id === props.selectedTunnelId ? 3.2 : 2.4,
      },
      zIndex: 3,
    }
  })
})

function handleNodeClick(event: { node: Node }) {
  const id = event.node.id.replace(/^tunnel-|^target-/, '')
  const tunnel = tunnelMap.value.get(id)
  if (tunnel) emit('select', tunnel)
}

// ---- Viewport control --------------------------------------------------------
// The canvas is locked horizontally (always centered) and may only scroll
// vertically. Zoom is clamped so it can never shrink past a usable level.
const clamp = (value: number, min: number, max: number) => Math.min(max, Math.max(min, value))
const clampZoom = (zoom: number) => clamp(zoom, MIN_ZOOM, MAX_ZOOM)

function centeredX(zoom: number): number {
  const width = dimensions.value.width || CONTENT_W
  return width / 2 - (CONTENT_W / 2) * zoom
}

function clampY(y: number, zoom: number): number {
  const height = dimensions.value.height || 0
  const scaledH = contentHeight.value * zoom
  // Content fits vertically => center it, no scrolling.
  if (scaledH + PAD_Y * 2 <= height) {
    return (height - scaledH) / 2
  }
  const top = PAD_Y
  const bottom = height - scaledH - PAD_Y
  return clamp(y, bottom, top)
}

function defaultZoom(): number {
  const width = dimensions.value.width || CONTENT_W
  return clampZoom(Math.min(1, (width - PAD_X * 2) / CONTENT_W))
}

let correcting = false

function apply(opts: { zoom?: number; y?: number; animate?: boolean } = {}) {
  if (!dimensions.value.width) return
  const zoom = clampZoom(opts.zoom ?? viewport.value.zoom)
  const y = clampY(opts.y ?? viewport.value.y, zoom)
  const x = centeredX(zoom)
  correcting = true
  void setViewport({ x, y, zoom }, opts.animate ? { duration: 200 } : undefined).finally(() => {
    correcting = false
  })
  persisted.zoom = zoom
  persisted.y = y
}

onInit(() => {
  apply({ zoom: persisted.zoom ?? defaultZoom(), y: persisted.y ?? PAD_Y })
})

onMove(() => {
  if (correcting) return
  const v = viewport.value
  const zoom = clampZoom(v.zoom)
  const targetX = centeredX(zoom)
  const targetY = clampY(v.y, zoom)
  persisted.zoom = zoom
  persisted.y = targetY
  if (Math.abs(v.x - targetX) > 0.5 || Math.abs(v.y - targetY) > 0.5 || Math.abs(v.zoom - zoom) > 0.001) {
    correcting = true
    void setViewport({ x: targetX, y: targetY, zoom }).finally(() => {
      correcting = false
    })
  }
})

// Re-center / re-clamp when the pane is resized.
watch(
  () => [dimensions.value.width, dimensions.value.height],
  () => apply(),
)
// Keep the current position stable when content changes (tunnels start/stop,
// rows added/removed) — re-clamp instead of resetting the view.
watch(contentHeight, () => apply())
</script>

<template>
  <div class="topology-view">
    <div v-if="isEmpty" class="empty-state">
      <svg class="empty-icon" viewBox="0 0 64 64" fill="none">
        <rect x="4" y="8" width="24" height="16" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <circle cx="24" cy="16" r="2" fill="#cbd5e1"/>
        <rect x="36" y="40" width="24" height="16" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <circle cx="56" cy="48" r="2" fill="#cbd5e1"/>
        <path d="M28 16 Q48 16 48 40" stroke="#e2e8f0" stroke-width="2" fill="none" stroke-dasharray="4 3"/>
      </svg>
      <p class="empty-title">{{ t('topology.noRemotesTitle') }}</p>
      <p class="empty-sub">{{ t('topology.noRemotesSub') }}</p>
    </div>
    <div v-else-if="!hasTunnels && !props.loading" class="empty-state">
      <svg class="empty-icon" viewBox="0 0 64 64" fill="none">
        <rect x="6" y="18" width="22" height="12" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <rect x="36" y="34" width="22" height="12" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <path d="M28 24 L36 40" stroke="#e2e8f0" stroke-width="2" stroke-dasharray="4 3"/>
      </svg>
      <p class="empty-title">{{ t('topology.noTunnelsTitle') }}</p>
      <p class="empty-sub">{{ t('topology.noTunnelsSub') }}</p>
    </div>
    <template v-else>
      <div class="remote-tabs">
        <button
          type="button"
          class="remote-tab remote-tab--all"
          :class="{ active: activeRemoteId === null }"
          @click="selectRemote(null)"
        >
          {{ t('topology.allRemotes') }}
        </button>
        <div class="remote-tab-scroll">
          <button
            v-for="remote in props.remotes"
            :key="remote.id"
            type="button"
            class="remote-tab"
            :class="{ active: activeRemoteId === remote.id }"
            :title="`${remote.host}:${remote.port}`"
            @click="selectRemote(remote.id)"
          >
            {{ remote.name }}
          </button>
        </div>
      </div>
      <VueFlow
        :nodes="flowNodes"
        :edges="flowEdges"
        :node-types="nodeTypes"
        :edge-types="edgeTypes"
        :min-zoom="MIN_ZOOM"
        :max-zoom="MAX_ZOOM"
        :nodes-draggable="false"
        :elements-selectable="false"
        :nodes-connectable="false"
        :pan-on-drag="false"
        :pan-on-scroll="true"
        :pan-on-scroll-mode="PanOnScrollMode.Vertical"
        :zoom-on-scroll="false"
        :zoom-on-double-click="false"
        :prevent-scrolling="true"
        class="flow"
        @node-click="handleNodeClick"
      >
        <Background pattern-color="#e2e8f0" :gap="20" />
        <Controls :show-fit-view="false" :show-interactive="false" position="bottom-right" />
      </VueFlow>
    </template>
  </div>
</template>

<style scoped>
.topology-view {
  width: 100%;
  height: calc(100vh - 240px);
  min-height: 560px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.remote-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  min-width: 0;
}

.remote-tab-scroll {
  display: flex;
  align-items: center;
  gap: 6px;
  overflow-x: auto;
  overflow-y: hidden;
  flex: 1;
  min-width: 0;
  padding-bottom: 2px;
  scrollbar-width: thin;
}

.remote-tab-scroll::-webkit-scrollbar { height: 6px; }
.remote-tab-scroll::-webkit-scrollbar-thumb { background: #cbd5e1; border-radius: 3px; }

.remote-tab {
  flex-shrink: 0;
  max-width: 180px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  color: #475569;
  background: #f1f5f9;
  border: 1px solid #e2e8f0;
  border-radius: 999px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.remote-tab:hover { background: #e2e8f0; }
.remote-tab.active {
  color: #fff;
  background: linear-gradient(135deg, #1d4ed8, #2563eb);
  border-color: #2563eb;
}

.remote-tab--all {
  flex-shrink: 0;
  position: sticky;
  left: 0;
}

.flow {
  flex: 1;
  min-height: 0;
  width: 100%;
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
}

.empty-state {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
}

.empty-icon { width: 72px; height: 72px; }
.empty-title { font-size: 15px; font-weight: 600; color: #64748b; }
.empty-sub { max-width: 420px; text-align: center; font-size: 13px; color: #94a3b8; }
</style>
