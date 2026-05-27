<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { VueFlow } from '@vue-flow/core'
import { Background } from '@vue-flow/background'
import { Controls } from '@vue-flow/controls'
import { NAlert, NButton, NSpin } from 'naive-ui'
import RemoteNode from '@/components/nodes/RemoteNode.vue'
import TunnelNode from '@/components/nodes/TunnelNode.vue'
import TargetNode from '@/components/nodes/TargetNode.vue'
import { api, type TopologyNode, type TopologyEdge } from '@/api/client'

import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'
import '@vue-flow/controls/dist/style.css'



const nodes = ref<TopologyNode[]>([])
const edges = ref<TopologyEdge[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)

const nodeTypes = {
  remote: RemoteNode,
  tunnel: TunnelNode,
  target: TargetNode
}

async function fetchTopology() {
  loading.value = true
  errorMsg.value = null
  try {
    const topo = await api.topology()
    nodes.value = topo.nodes
    edges.value = topo.edges.map(e => ({
      ...e,
      animated: !!e.animated
    }))
  } catch (e: any) {
    errorMsg.value = e.message
  } finally {
    loading.value = false
  }
}

let interval: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchTopology()
  interval = setInterval(fetchTopology, 5000)
})

onUnmounted(() => {
  if (interval) clearInterval(interval)
})
</script>

<template>
  <div class="dashboard">
    <div class="toolbar">
      <h2>Topology</h2>
      <n-button size="small" @click="fetchTopology" :loading="loading">Refresh</n-button>
    </div>
    <n-alert v-if="errorMsg" type="error" :title="errorMsg" style="margin: 12px; flex-shrink: 0" />
    <div class="flow-container">
      <n-spin :show="loading && nodes.length === 0">
        <VueFlow
          v-if="nodes.length > 0 || !loading"
          :nodes="nodes"
          :edges="edges"
          :node-types="nodeTypes"
          fit-view-on-init
          class="vue-flow"
        >
          <Background />
          <Controls />
        </VueFlow>
        <div v-if="nodes.length === 0 && !loading" class="empty-state">
          <p>No tunnels configured yet.</p>
          <p>Add a <a href="/remotes">remote</a> and <a href="/tunnels">tunnel</a> to get started.</p>
        </div>
      </n-spin>
    </div>
    <div class="legend">
      <span class="legend-item remote">🖥 Remote SSH server</span>
      <span class="legend-item tunnel">🔌 Tunnel rule</span>
      <span class="legend-item target">📍 Target endpoint</span>
    </div>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid #e5e7eb;
}
.toolbar h2 { font-size: 16px; font-weight: 600; }
.flow-container {
  flex: 1;
  position: relative;
  overflow: hidden;
}
.vue-flow { width: 100%; height: 100%; }
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #9ca3af;
  gap: 8px;
}
.legend {
  display: flex;
  gap: 24px;
  padding: 8px 16px;
  border-top: 1px solid #e5e7eb;
  font-size: 12px;
  color: #6b7280;
}
</style>
