<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

const props = defineProps<{
  data: {
    label: string
    host: string
    port: number
    user?: string
  }
}>()
</script>

<template>
  <div class="remote-node">
    <div class="node-header">
      <span class="icon">🖥</span>
      <span class="label">{{ data.label }}</span>
    </div>
    <div class="node-body">
      <div class="info-row">
        <span class="key">host</span>
        <span class="value">{{ data.host }}:{{ data.port }}</span>
      </div>
      <div v-if="data.user" class="info-row">
        <span class="key">user</span>
        <span class="value">{{ data.user }}</span>
      </div>
    </div>
    <Handle type="target" :position="Position.Left" />
    <Handle type="source" :position="Position.Right" />
  </div>
</template>

<style scoped>
.remote-node {
  background: #1a56db20;
  border: 2px solid #1a56db;
  border-radius: 8px;
  min-width: 180px;
  font-size: 12px;
  overflow: hidden;
}
.node-header {
  background: #1a56db;
  color: white;
  padding: 6px 10px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
  font-size: 13px;
}
.node-body {
  padding: 8px 10px;
}
.info-row { display: flex; justify-content: space-between; gap: 8px; margin-bottom: 2px; }
.key { color: #6b7280; }
.value { font-family: monospace; color: #111827; }
</style>
