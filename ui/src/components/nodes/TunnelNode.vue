<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

const props = defineProps<{
  data: {
    label: string
    direction: string
    bindAddress: string
    bindPort: number
    state: string
  }
}>()

const stateColor: Record<string, string> = {
  running: '#16a34a',
  stopped: '#6b7280',
  error: '#dc2626'
}
</script>

<template>
  <div class="tunnel-node" :class="`state-${data.state}`">
    <div class="node-header">
      <span class="icon">🔌</span>
      <span class="label">{{ data.label }}</span>
      <span class="dir-badge">{{ data.direction }}</span>
    </div>
    <div class="node-body">
      <div class="info-row">
        <span class="key">bind</span>
        <span class="value">{{ data.bindAddress }}:{{ data.bindPort }}</span>
      </div>
      <div class="info-row">
        <span class="key">state</span>
        <span class="value state-text" :style="{ color: stateColor[data.state] || '#6b7280' }">
          {{ data.state }}
        </span>
      </div>
    </div>
    <Handle type="source" :position="Position.Left" id="left" />
    <Handle type="source" :position="Position.Right" id="right" />
    <Handle type="target" :position="Position.Top" id="top" />
    <Handle type="target" :position="Position.Bottom" id="bottom" />
  </div>
</template>

<style scoped>
.tunnel-node {
  border: 2px solid #d97706;
  border-radius: 8px;
  min-width: 180px;
  font-size: 12px;
  overflow: hidden;
  background: #fef3c710;
}
.tunnel-node.state-running { border-color: #16a34a; background: #f0fdf410; }
.tunnel-node.state-error   { border-color: #dc2626; background: #fef2f210; }
.node-header {
  background: #d97706;
  color: white;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
}
.state-running .node-header { background: #16a34a; }
.state-error .node-header   { background: #dc2626; }
.dir-badge {
  margin-left: auto;
  background: rgba(255,255,255,0.3);
  border-radius: 4px;
  padding: 1px 5px;
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
}
.node-body { padding: 8px 10px; }
.info-row { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 2px; }
.key { color: #6b7280; }
.value { font-family: monospace; color: #111827; }
</style>
