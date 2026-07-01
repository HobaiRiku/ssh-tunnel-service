<script setup lang="ts">
import { computed, markRaw, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { MarkerType, PanOnScrollMode, VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { useMessage, useOsTheme } from 'naive-ui'
import type { Edge, Node } from '@vue-flow/core'
import { api, getErrorMessage, type Remote, type TunnelStatus } from '@/api/client'
import TunnelEdge from '@/components/edges/TunnelEdge.vue'
import RemoteGroupNode from '@/components/nodes/RemoteGroupNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'
import { topologyViewState } from '@/components/topologyViewState'
import { copyText } from '@/clipboard'
import { useI18n } from '@/i18n'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

const props = defineProps<{
  tunnels: TunnelStatus[]
  remotes: Remote[]
  loading: boolean
  selectedTunnelName?: string | null
}>()

const emit = defineEmits<{
  (e: 'select', tunnel: TunnelStatus): void
}>()

const { viewport, setViewport, dimensions, onInit, onMove } = useVueFlow()
const { t } = useI18n()
const message = useMessage()

// VueFlow renders edge/marker colors as raw SVG attributes rather than through
// the CSS cascade, so they can't pick up the app's --color-* custom properties;
// resolve concrete hex values from the OS theme instead.
const osTheme = useOsTheme()
const isDark = computed(() => osTheme.value === 'dark')
const flowPalette = computed(() => (isDark.value
  ? { accent: '#3b82f6', success: '#22c55e', danger: '#ef4444', muted: '#64748b', border: '#475569', borderSoft: '#334155' }
  : { accent: '#2563eb', success: '#22c55e', danger: '#ef4444', muted: '#94a3b8', border: '#cbd5e1', borderSoft: '#e2e8f0' }
))

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

const tunnelMap = computed(() => new Map(props.tunnels.map((tunnel) => [tunnel.name, tunnel] as const)))

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
  const found = props.remotes.filter((remote) => remote.name === activeRemoteId.value)
  return found.length > 0 ? found : props.remotes
})

const visibleRemoteNames = computed(() => new Set(filteredRemotes.value.map((remote) => remote.name)))
const visibleTunnels = computed(() => props.tunnels.filter((tunnel) => visibleRemoteNames.value.has(tunnel.remote)))

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
    if (activeRemoteId.value && !remotes.some((remote) => remote.name === activeRemoteId.value)) {
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
    const remoteTunnels = props.tunnels.filter((tunnel) => tunnel.remote === remote.name)
    const slotCount = Math.max(remoteTunnels.length, 1)
    const groupH = GROUP_HEADER_H + GROUP_PAD_TOP + slotCount * TUNNEL_SLOT_H + GROUP_PAD_BOT

    nodes.push({
      id: `group-${remote.name}`,
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
        id: `tunnel-${tunnel.name}`,
        type: 'tunnel',
        parentNode: `group-${remote.name}`,
        extent: 'parent',
        position: { x: TUNNEL_INDENT, y: childY },
        data: {
          label: tunnel.name,
          direction: tunnel.direction,
          remoteHost: remoteEndpoint.host,
          remotePort: remoteEndpoint.port,
          localHost: localEndpoint.host,
          localPort: localEndpoint.port,
          state: tunnel.state,
          selected: tunnel.name === props.selectedTunnelName,
          onSelect,
        },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })

      nodes.push({
        id: `target-${tunnel.name}`,
        type: 'target',
        position: { x: TARGET_X, y: absY + 38 },
        data: {
          host: localEndpoint.host,
          port: localEndpoint.port,
          selected: tunnel.name === props.selectedTunnelName,
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
    const selected = tunnel.name === props.selectedTunnelName
    const palette = flowPalette.value
    const stroke = selected
      ? palette.accent
      : tunnel.state === 'running'
        ? palette.success
        : tunnel.state === 'error'
          ? palette.danger
          : palette.muted

    // A -R (remote forward) flows from the remote side back to the local
    // target, so the arrow points the opposite way: anchor it at the source
    // end (markerStart) instead of the target end (markerEnd).
    const reversed = tunnel.direction === '-R'
    const marker = { type: MarkerType.ArrowClosed, width: 22, height: 22, strokeWidth: 1, color: stroke, markerUnits: 'userSpaceOnUse' as const }

    return {
      id: `edge-${tunnel.name}`,
      source: `tunnel-${tunnel.name}`,
      target: `target-${tunnel.name}`,
      type: 'tunnel',
      animated: tunnel.state === 'running',
      data: {
        remotePort: remoteEndpoint.port,
        localPort: localEndpoint.port,
        selected,
        reversed,
      },
      markerStart: reversed ? marker : undefined,
      markerEnd: reversed ? undefined : marker,
      style: {
        stroke,
        strokeWidth: selected ? 3.2 : 2.4,
      },
      zIndex: 3,
    }
  })
})

function tunnelFromNodeId(nodeId: string): TunnelStatus | undefined {
  if (nodeId.startsWith('tunnel-')) return tunnelMap.value.get(nodeId.slice('tunnel-'.length))
  if (nodeId.startsWith('target-')) return tunnelMap.value.get(nodeId.slice('target-'.length))
  return undefined
}

function handleNodeClick(event: { node: Node }) {
  const tunnel = tunnelFromNodeId(event.node.id)
  if (tunnel) emit('select', tunnel)
}

// ---- Right-click context menu (copy helpers) ---------------------------------
const menu = reactive<{ visible: boolean; x: number; y: number; tunnel: TunnelStatus | null }>({
  visible: false,
  x: 0,
  y: 0,
  tunnel: null,
})

function onNodeContextMenu({ event, node }: { event: MouseEvent; node: Node }) {
  const tunnel = tunnelFromNodeId(node.id)
  if (!tunnel) return
  event.preventDefault()
  menu.visible = true
  menu.x = event.clientX
  menu.y = event.clientY
  menu.tunnel = tunnel
}

function closeMenu() {
  menu.visible = false
  menu.tunnel = null
}

async function copyValue(value: string) {
  const ok = await copyText(value)
  if (ok) message.success(t('common.copied'))
  else message.error(t('common.copyFailed'))
  closeMenu()
}

function copyMenuName() {
  if (menu.tunnel) void copyValue(menu.tunnel.name)
}

function copyMenuBind() {
  if (menu.tunnel) void copyValue(`${menu.tunnel.bind_address}:${menu.tunnel.bind_port}`)
}

function copyMenuTarget() {
  if (menu.tunnel) void copyValue(`${menu.tunnel.target_host}:${menu.tunnel.target_port}`)
}

async function copyMenuCommand() {
  if (!menu.tunnel) return
  const name = menu.tunnel.name
  closeMenu()
  try {
    const preview = await api.getTunnelCommand(name)
    const ok = await copyText(preview.command)
    if (ok) message.success(t('common.copied'))
    else message.error(t('common.copyFailed'))
  } catch (err: unknown) {
    message.error(getErrorMessage(err))
  }
}

if (typeof window !== 'undefined') {
  window.addEventListener('click', closeMenu)
  window.addEventListener('scroll', closeMenu, true)
  onBeforeUnmount(() => {
    window.removeEventListener('click', closeMenu)
    window.removeEventListener('scroll', closeMenu, true)
  })
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
        <rect x="4" y="8" width="24" height="16" rx="3" :stroke="flowPalette.border" stroke-width="2"/>
        <circle cx="24" cy="16" r="2" :fill="flowPalette.border"/>
        <rect x="36" y="40" width="24" height="16" rx="3" :stroke="flowPalette.border" stroke-width="2"/>
        <circle cx="56" cy="48" r="2" :fill="flowPalette.border"/>
        <path d="M28 16 Q48 16 48 40" :stroke="flowPalette.borderSoft" stroke-width="2" fill="none" stroke-dasharray="4 3"/>
      </svg>
      <p class="empty-title">{{ t('topology.noRemotesTitle') }}</p>
      <p class="empty-sub">{{ t('topology.noRemotesSub') }}</p>
    </div>
    <div v-else-if="!hasTunnels && !props.loading" class="empty-state">
      <svg class="empty-icon" viewBox="0 0 64 64" fill="none">
        <rect x="6" y="18" width="22" height="12" rx="3" :stroke="flowPalette.border" stroke-width="2"/>
        <rect x="36" y="34" width="22" height="12" rx="3" :stroke="flowPalette.border" stroke-width="2"/>
        <path d="M28 24 L36 40" :stroke="flowPalette.borderSoft" stroke-width="2" stroke-dasharray="4 3"/>
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
            :key="remote.name"
            type="button"
            class="remote-tab"
            :class="{ active: activeRemoteId === remote.name }"
            :title="`${remote.host}:${remote.port}`"
            @click="selectRemote(remote.name)"
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
        @node-context-menu="onNodeContextMenu"
      >
        <Background :pattern-color="flowPalette.borderSoft" :gap="20" />
        <Controls :show-fit-view="false" :show-interactive="false" position="bottom-right" />
      </VueFlow>
    </template>

    <div
      v-if="menu.visible"
      class="ctx-menu"
      :style="{ left: menu.x + 'px', top: menu.y + 'px' }"
      @click.stop
      @contextmenu.prevent
    >
      <div class="ctx-menu-title">{{ menu.tunnel?.name }}</div>
      <button type="button" class="ctx-menu-item" @click="copyMenuName">{{ t('topology.copyName') }}</button>
      <button type="button" class="ctx-menu-item" @click="copyMenuBind">{{ t('topology.copyBind') }}</button>
      <button type="button" class="ctx-menu-item" @click="copyMenuTarget">{{ t('topology.copyTarget') }}</button>
      <button type="button" class="ctx-menu-item" @click="copyMenuCommand">{{ t('topology.copyCommand') }}</button>
    </div>
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
.remote-tab-scroll::-webkit-scrollbar-thumb { background: var(--color-border-strong); border-radius: 3px; }

.remote-tab {
  flex-shrink: 0;
  max-width: 180px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 999px;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.remote-tab:hover { background: var(--color-border); }
.remote-tab.active {
  color: #fff;
  background: linear-gradient(135deg, var(--color-accent-strong), var(--color-accent));
  border-color: var(--color-accent);
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
  background: linear-gradient(180deg, var(--color-flow-from) 0%, var(--color-flow-to) 100%);
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
  background: linear-gradient(180deg, var(--color-flow-from) 0%, var(--color-flow-to) 100%);
  border-radius: 12px;
}

.empty-icon { width: 72px; height: 72px; }
.empty-title { font-size: 15px; font-weight: 600; color: var(--color-text-tertiary); }
.empty-sub { max-width: 420px; text-align: center; font-size: 13px; color: var(--color-text-muted); }

.ctx-menu {
  position: fixed;
  z-index: 9999;
  min-width: 168px;
  padding: 4px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 10px;
  box-shadow: 0 10px 30px var(--color-shadow-strong);
}
.ctx-menu-title {
  padding: 6px 10px 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 240px;
  border-bottom: 1px solid var(--color-border);
  margin-bottom: 4px;
}
.ctx-menu-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 7px 10px;
  border: none;
  background: transparent;
  border-radius: 6px;
  font-size: 13px;
  color: var(--color-text);
  cursor: pointer;
}
.ctx-menu-item:hover { background: var(--color-surface-alt); }
</style>
