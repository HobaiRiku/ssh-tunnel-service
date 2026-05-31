<script setup lang="ts">
import { computed, markRaw, nextTick, onMounted, watch } from 'vue'
import { MarkerType, VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import type { Edge, Node } from '@vue-flow/core'
import type { Remote, TunnelStatus } from '@/api/client'
import TunnelEdge from '@/components/edges/TunnelEdge.vue'
import RemoteGroupNode from '@/components/nodes/RemoteGroupNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'
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

const { fitView } = useVueFlow()
const { t } = useI18n()

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

const isEmpty = computed(() => !props.loading && props.remotes.length === 0)
const hasTunnels = computed(() => props.tunnels.length > 0)

const flowNodes = computed<Node[]>(() => {
  const nodes: Node[] = []
  let yOffset = 0

  for (const remote of props.remotes) {
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

      nodes.push({
        id: `tunnel-${tunnel.id}`,
        type: 'tunnel',
        parentNode: `group-${remote.id}`,
        extent: 'parent',
        position: { x: TUNNEL_INDENT, y: childY },
        data: {
          label: tunnel.name || tunnel.id,
          direction: tunnel.direction,
          bindAddress: tunnel.bind_address,
          bindPort: tunnel.bind_port,
          targetHost: tunnel.target_host,
          targetPort: tunnel.target_port,
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
        data: { host: tunnel.target_host, port: tunnel.target_port, selected: tunnel.id === props.selectedTunnelId, onSelect },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })
    })

    yOffset += groupH + GROUP_GAP
  }

  return nodes
})

const flowEdges = computed<Edge[]>(() => {
  return props.tunnels.map((tunnel) => {
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
        direction: tunnel.direction,
        bindPort: tunnel.bind_port,
        targetPort: tunnel.target_port,
        selected: tunnel.id === props.selectedTunnelId,
      },
      markerStart: tunnel.direction === '-R'
        ? { type: MarkerType.ArrowClosed, width: 12, height: 12, strokeWidth: 1.25, color: stroke }
        : undefined,
      markerEnd: tunnel.direction === '-L'
        ? { type: MarkerType.ArrowClosed, width: 12, height: 12, strokeWidth: 1.25, color: stroke }
        : undefined,
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

async function refit() {
  await nextTick()
  await new Promise<void>((resolve) => {
    window.setTimeout(resolve, 60)
  })
  if (flowNodes.value.length > 0) {
    await fitView({ padding: 0.18, duration: 200 })
  }
}

watch(() => flowNodes.value.length, () => {
  void refit()
})
watch(() => props.loading, (loading) => {
  if (!loading) void refit()
})
onMounted(() => {
  void refit()
})
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
      <div class="topology-toolbar">
        <span>{{ t('topology.clickHint') }}</span>
      </div>
      <VueFlow
        :nodes="flowNodes"
        :edges="flowEdges"
        :node-types="nodeTypes"
        :edge-types="edgeTypes"
        fit-view-on-init
        :min-zoom="0.3"
        :max-zoom="2"
        :nodes-draggable="false"
        :elements-selectable="false"
        :nodes-connectable="false"
        class="flow"
        @node-click="handleNodeClick"
      >
        <Background pattern-color="#e2e8f0" :gap="20" />
        <Controls />
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

.topology-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  padding: 10px 14px;
  background: #eff6ff;
  color: #1d4ed8;
  border-radius: 12px;
  font-size: 13px;
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
