<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{
  data: {
    host: string
    port: number
  }
}>()
</script>

<template>
  <div class="target-node">
    <Handle type="target" :position="Position.Left" class="handle-left" />
    <div class="target-inner">
      <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8">
        <circle cx="10" cy="10" r="8"/>
        <circle cx="10" cy="10" r="4.5"/>
        <circle cx="10" cy="10" r="1.5" fill="currentColor" stroke="none"/>
      </svg>
      <div class="addr-block">
        <span class="host">{{ data.host }}</span>
        <span class="sep">:</span>
        <span class="port">{{ data.port }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.target-node {
  display: inline-flex;
  align-items: center;
  background: #faf5ff;
  border: 1.5px solid #7c3aed;
  border-radius: 8px;
  box-shadow: 0 1px 4px rgba(0,0,0,0.08);
  overflow: hidden;
}

.target-inner {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 12px;
}

.icon {
  width: 16px;
  height: 16px;
  color: #7c3aed;
  flex-shrink: 0;
}

.addr-block {
  display: flex;
  align-items: baseline;
  gap: 1px;
  font-family: 'SF Mono', 'Fira Code', monospace;
}

.host { font-size: 11px; color: #4c1d95; font-weight: 500; }
.sep  { font-size: 11px; color: #7c3aed; }
.port { font-size: 12px; color: #5b21b6; font-weight: 700; }

:deep(.handle-left) {
  width: 10px !important;
  height: 10px !important;
  background: #7c3aed !important;
  border: 2px solid white !important;
}
</style>
