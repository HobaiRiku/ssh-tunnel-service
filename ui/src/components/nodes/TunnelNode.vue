<script setup lang="ts">
import { computed } from 'vue'
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

const stateStyle = computed(() => {
  switch (props.data.state) {
    case 'running': return { bg: '#f0fdf4', border: '#22c55e', dot: '#22c55e' }
    case 'error':   return { bg: '#fef2f2', border: '#ef4444', dot: '#ef4444' }
    default:        return { bg: '#f8fafc', border: '#cbd5e1', dot: '#94a3b8' }
  }
})
</script>

<template>
  <div
    class="tunnel-node"
    :style="{ background: stateStyle.bg, borderColor: stateStyle.border }"
  >
    <div class="tunnel-top">
      <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
        <path d="M3 10h14M11 5l6 5-6 5"/>
      </svg>
      <span class="name">{{ data.label }}</span>
      <span class="state-dot" :style="{ background: stateStyle.dot }"></span>
    </div>
    <div class="tunnel-info">
      <span class="dir-badge" :class="data.direction === '-L' ? 'local' : 'remote'">{{ data.direction }}</span>
      <span class="addr">{{ data.bindAddress }}:{{ data.bindPort }}</span>
    </div>
    <Handle type="source" :position="Position.Right" class="handle-right" />
  </div>
</template>

<style scoped>
.tunnel-node {
  border: 1.5px solid;
  border-radius: 8px;
  width: 190px;
  overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,0.08);
}

.tunnel-top {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 10px 5px;
  border-bottom: 1px solid rgba(0,0,0,0.06);
}

.icon { width: 14px; height: 14px; color: #64748b; flex-shrink: 0; }

.name {
  font-weight: 600;
  font-size: 12px;
  color: #1e293b;
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.state-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.tunnel-info {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px 7px;
}

.dir-badge {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 10px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 4px;
  flex-shrink: 0;
}
.dir-badge.local  { background: #dbeafe; color: #1d4ed8; }
.dir-badge.remote { background: #fce7f3; color: #9d174d; }

.addr {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 10.5px;
  color: #475569;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.handle-right) {
  width: 10px !important;
  height: 10px !important;
  background: #2563eb !important;
  border: 2px solid white !important;
}
</style>
