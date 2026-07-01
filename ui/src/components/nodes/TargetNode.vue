<script setup lang="ts">
import { Handle, Position } from '@vue-flow/core'

defineProps<{
  data: {
    host: string
    port: number
    selected?: boolean
    onSelect?: () => void
  }
}>()
</script>

<template>
  <div class="target-node" :class="{ selected: data.selected }" role="button" tabindex="0" @click.stop="data.onSelect?.()" @keydown.enter.stop="data.onSelect?.()" @keydown.space.stop="data.onSelect?.()">
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
.target-node { display: inline-flex; align-items: center; background: var(--color-node-target-bg); border: 1.5px solid var(--color-node-target-border); border-radius: 10px; box-shadow: 0 1px 4px var(--color-shadow-soft); overflow: visible; cursor: pointer; }
.target-node.selected { border-color: var(--color-accent); box-shadow: 0 8px 18px var(--color-shadow-medium); }
.target-inner { display: flex; align-items: center; gap: 7px; padding: 10px 14px; }
.icon { width: 16px; height: 16px; color: var(--color-node-target-border); flex-shrink: 0; }
.addr-block { display: flex; align-items: baseline; gap: 1px; font-family: 'SF Mono', 'Fira Code', monospace; }
.host { font-size: 11px; color: var(--color-node-target-text); font-weight: 500; }
.sep { font-size: 11px; color: var(--color-node-target-border); }
.port { font-size: 12px; color: var(--color-node-target-text-strong); font-weight: 700; }
:deep(.handle-left) {
  width: 10px !important;
  height: 10px !important;
  left: -5px;
  background: var(--color-node-target-border) !important;
  border: 2px solid var(--color-surface) !important;
  z-index: 4;
  animation: target-handle-pulse 1.8s ease-out infinite;
}

@keyframes target-handle-pulse {
  0% { box-shadow: 0 0 0 0 rgba(124, 58, 237, 0.55); }
  70% { box-shadow: 0 0 0 10px rgba(124, 58, 237, 0); }
  100% { box-shadow: 0 0 0 0 rgba(124, 58, 237, 0); }
}
</style>
