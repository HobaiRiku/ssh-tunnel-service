<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { VueFlow, useVueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { NButton, NSpin } from 'naive-ui'
import type { Node, Edge } from '@vue-flow/core'
import RemoteGroupNode from '@/components/nodes/RemoteGroupNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'
import { useTunnelsStore } from '@/stores/tunnels'
import { useRemotesStore } from '@/stores/remotes'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'

const tunnelsStore = useTunnelsStore()
const remotesStore = useRemotesStore()

const { fitView } = useVueFlow()

const nodeTypes = {
  remoteGroup: RemoteGroupNode,
  tunnel: TunnelNode,
  target: TargetNode,
}

const GROUP_HEADER_H = 52
const TUNNEL_SLOT_H  = 90
const GROUP_PAD_TOP  = 12
const GROUP_PAD_BOT  = 12
const GROUP_W        = 220
const TUNNEL_INDENT  = 12
const TARGET_X       = 400

const flowNodes = computed<Node[]>(() => {
  const nodes: Node[] = []
  let yOffset = 0

  for (const remote of remotesStore.remotes) {
    const rtunnels = tunnelsStore.tunnels.filter(t => t.remote_id === remote.id)
    const slotCount = Math.max(rtunnels.length, 1)
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

    rtunnels.forEach((tunnel, i) => {
      const childY = GROUP_HEADER_H + GROUP_PAD_TOP + i * TUNNEL_SLOT_H
      const absY   = yOffset + childY

      nodes.push({
        id: `tunnel-${tunnel.id}`,
        type: 'tunnel',
        parentNode: `group-${remote.id}`,
        extent: 'parent',
        position: { x: TUNNEL_INDENT, y: childY },
        data: {
          label: tunnel.name,
          direction: tunnel.direction,
          bindAddress: tunnel.bind_address,
          bindPort: tunnel.bind_port,
          state: tunnel.state,
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
  return tunnelsStore.tunnels.map(tunnel => ({
    id: `e-${tunnel.id}`,
    source: `tunnel-${tunnel.id}`,
    target: `target-${tunnel.id}`,
    type: 'smoothstep',
    animated: tunnel.state === 'running',
    label: tunnel.direction === '-L'
      ? `→ :${tunnel.target_port}`
      : `← :${tunnel.bind_port}`,
    labelStyle: { fontSize: '10px', fill: '#64748b' },
    labelBgStyle: { fill: 'transparent' },
    style: {
      stroke: tunnel.state === 'running' ? '#22c55e'
            : tunnel.state === 'error'   ? '#ef4444'
            : '#94a3b8',
      strokeWidth: 1.5,
    },
  }))
})

const isEmpty = computed(() =>
  !tunnelsStore.loading && !remotesStore.loading &&
  remotesStore.remotes.length === 0
)
const isLoading = computed(() => tunnelsStore.loading || remotesStore.loading)

async function refresh() {
  await Promise.all([tunnelsStore.fetchTunnels(), remotesStore.fetchRemotes()])
  setTimeout(() => fitView({ padding: 0.15, duration: 300 }), 50)
}

watch(flowNodes, () => {
  setTimeout(() => fitView({ padding: 0.15, duration: 200 }), 50)
}, { once: true })

let interval: ReturnType<typeof setInterval> | null = null
onMounted(() => {
  refresh()
  interval = setInterval(refresh, 5000)
})
onUnmounted(() => { if (interval) clearInterval(interval) })
</script>

<template>
  <div class="topology-page">
    <div class="topo-toolbar">
      <span class="toolbar-title">Topology</span>
      <n-button size="small" :loading="isLoading" @click="refresh">Refresh</n-button>
    </div>

    <div class="flow-wrap">
      <n-spin :show="isLoading && flowNodes.length === 0" style="height:100%">
        <div v-if="isEmpty" class="empty-state">
          <svg class="empty-icon" viewBox="0 0 64 64" fill="none">
            <rect x="4" y="8" width="24" height="16" rx="3" stroke="#cbd5e1" stroke-width="2"/>
            <circle cx="24" cy="16" r="2" fill="#cbd5e1"/>
            <rect x="36" y="40" width="24" height="16" rx="3" stroke="#cbd5e1" stroke-width="2"/>
            <circle cx="56" cy="48" r="2" fill="#cbd5e1"/>
            <path d="M28 16 Q48 16 48 40" stroke="#e2e8f0" stroke-width="2" fill="none" stroke-dasharray="4 3"/>
          </svg>
          <p class="empty-title">No tunnels yet</p>
          <p class="empty-sub">
            Add a <a href="/remotes">Remote</a> and a <a href="/tunnels">Tunnel</a> to visualize the topology.
          </p>
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
      </n-spin>
    </div>
  </div>
</template>

<style scoped>
.topology-page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.topo-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 20px;
  background: #fff;
  border-bottom: 1px solid #e2e8f0;
  flex-shrink: 0;
}

.toolbar-title {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.flow-wrap {
  flex: 1;
  min-height: 0;
  position: relative;
}

.flow {
  width: 100%;
  height: 100%;
}

.empty-state {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.empty-icon { width: 72px; height: 72px; }

.empty-title {
  font-size: 15px;
  font-weight: 600;
  color: #64748b;
}

.empty-sub {
  font-size: 13px;
  color: #94a3b8;
}

.empty-sub a { color: #2563eb; text-decoration: none; }
.empty-sub a:hover { text-decoration: underline; }
</style>
