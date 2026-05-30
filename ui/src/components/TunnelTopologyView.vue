<script setup lang="ts">
import { computed, nextTick, onMounted, watch } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import type { Edge, Node } from '@vue-flow/core'
import type { Remote, TunnelStatus } from '@/api/client'
import RemoteGroupNode from '@/components/nodes/RemoteGroupNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

const props = defineProps<{
  tunnels: TunnelStatus[]
  remotes: Remote[]
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'start', id: string): void
  (e: 'stop', id: string): void
  (e: 'edit', tunnel: TunnelStatus): void
  (e: 'delete', id: string): void
  (e: 'command', tunnel: TunnelStatus): void
}>()

const { fitView } = useVueFlow()

const nodeTypes = {
  remoteGroup: RemoteGroupNode,
  tunnel: TunnelNode,
  target: TargetNode,
}

const GROUP_HEADER_H = 52
const TUNNEL_SLOT_H = 116
const GROUP_PAD_TOP = 12
const GROUP_PAD_BOT = 12
const GROUP_W = 252
const TUNNEL_INDENT = 12
const TARGET_X = 456

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
          onStart: () => emit('start', tunnel.id),
          onStop: () => emit('stop', tunnel.id),
          onEdit: () => emit('edit', tunnel),
          onDelete: () => emit('delete', tunnel.id),
          onCommand: () => emit('command', tunnel),
        },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })

      nodes.push({
        id: `target-${tunnel.id}`,
        type: 'target',
        position: { x: TARGET_X, y: absY + (TUNNEL_SLOT_H - 36) / 2 },
        data: { host: tunnel.target_host, port: tunnel.target_port },
        draggable: false,
        selectable: false,
        zIndex: 1,
      })
    })

    yOffset += groupH + 24
  }

  return nodes
})

const flowEdges = computed<Edge[]>(() => {
  return props.tunnels.map((tunnel) => ({
    id: `edge-${tunnel.id}`,
    source: `tunnel-${tunnel.id}`,
    target: `target-${tunnel.id}`,
    type: 'smoothstep',
    animated: tunnel.state === 'running',
    label: tunnel.direction === '-L' ? `→ :${tunnel.target_port}` : `← :${tunnel.bind_port}`,
    labelStyle: { fontSize: '10px', fill: '#64748b' },
    labelBgStyle: { fill: 'transparent' },
    style: {
      stroke: tunnel.state === 'running' ? '#22c55e' : tunnel.state === 'error' ? '#ef4444' : '#94a3b8',
      strokeWidth: 1.5,
    },
  }))
})

function refit() {
  nextTick(() => {
    setTimeout(() => {
      if (flowNodes.value.length > 0) {
        fitView({ padding: 0.15, duration: 200 })
      }
    }, 60)
  })
}

watch(() => flowNodes.value.length, refit)
watch(() => props.loading, (loading) => {
  if (!loading) {
    refit()
  }
})
onMounted(refit)
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
      <p class="empty-title">No remotes yet</p>
      <p class="empty-sub">Create a remote and then add tunnels to view the topology here.</p>
    </div>
    <div v-else-if="!hasTunnels && !props.loading" class="empty-state">
      <svg class="empty-icon" viewBox="0 0 64 64" fill="none">
        <rect x="6" y="18" width="22" height="12" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <rect x="36" y="34" width="22" height="12" rx="3" stroke="#cbd5e1" stroke-width="2"/>
        <path d="M28 24 L36 40" stroke="#e2e8f0" stroke-width="2" stroke-dasharray="4 3"/>
      </svg>
      <p class="empty-title">No tunnels yet</p>
      <p class="empty-sub">Add a tunnel to render the topology and manage it directly from this view.</p>
    </div>
    <VueFlow
      v-else
      :nodes="flowNodes"
      :edges="flowEdges"
      :node-types="nodeTypes"
      fit-view-on-init
      :min-zoom="0.3"
      :max-zoom="2"
      class="flow"
    >
      <Background pattern-color="#e2e8f0" :gap="20" />
      <Controls />
    </VueFlow>
  </div>
</template>

<style scoped>
.topology-view {
  height: 100%;
  min-height: 520px;
}

.flow {
  width: 100%;
  height: 100%;
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
}

.empty-state {
  height: 100%;
  min-height: 520px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
  border-radius: 12px;
}

.empty-icon {
  width: 72px;
  height: 72px;
}

.empty-title {
  font-size: 15px;
  font-weight: 600;
  color: #64748b;
}

.empty-sub {
  max-width: 420px;
  text-align: center;
  font-size: 13px;
  color: #94a3b8;
}
</style>
