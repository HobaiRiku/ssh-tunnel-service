<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from '@/i18n'

const props = defineProps<{
  data: {
    label: string
    direction: string
    bindAddress: string
    bindPort: number
    targetHost: string
    targetPort: number
    state: string
    selected?: boolean
    onSelect?: () => void
  }
}>()

const { t } = useI18n()

const stateStyle = computed(() => {
  switch (props.data.state) {
    case 'running':
      return { bg: '#f0fdf4', border: '#22c55e', dot: '#22c55e', label: t('common.running') }
    case 'error':
      return { bg: '#fef2f2', border: '#ef4444', dot: '#ef4444', label: t('common.error') }
    default:
      return { bg: '#f8fafc', border: '#cbd5e1', dot: '#94a3b8', label: t('common.stopped') }
  }
})
</script>

<template>
  <div
    class="tunnel-node"
    :class="{ selected: data.selected }"
    :style="{ background: stateStyle.bg, borderColor: stateStyle.border }"
    role="button"
    tabindex="0"
    @click.stop="data.onSelect?.()"
    @keydown.enter.stop="data.onSelect?.()"
    @keydown.space.stop="data.onSelect?.()"
  >
    <div class="tunnel-top">
      <svg class="icon" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
        <path d="M3 10h14M11 5l6 5-6 5"/>
      </svg>
      <span class="name">{{ data.label }}</span>
      <span class="state-pill" :style="{ background: stateStyle.bg, color: stateStyle.dot, borderColor: stateStyle.border }">
        <span class="state-dot" :style="{ background: stateStyle.dot }"></span>
        {{ stateStyle.label }}
      </span>
    </div>
    <div class="tunnel-info">
      <div class="line">
        <span class="dir-badge" :class="data.direction === '-L' ? 'local' : 'remote'">{{ data.direction }}</span>
        <span class="addr">{{ data.bindAddress }}:{{ data.bindPort }}</span>
      </div>
      <div class="target">→ {{ data.targetHost }}:{{ data.targetPort }}</div>
    </div>
    <Handle type="source" :position="Position.Right" class="handle-right" />
  </div>
</template>

<style scoped>
.tunnel-node {
  border: 1.5px solid;
  border-radius: 10px;
  width: 272px;
  overflow: visible;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  cursor: pointer;
  transition: box-shadow 0.15s ease, transform 0.15s ease, border-color 0.15s ease;
}

.tunnel-node:hover,
.tunnel-node.selected {
  box-shadow: 0 8px 18px rgba(15, 23, 42, 0.14);
  transform: translateY(-1px);
}

.tunnel-node.selected { border-color: #2563eb !important; }
.tunnel-node:focus-visible { outline: 2px solid #2563eb; outline-offset: 2px; }
.tunnel-top { display: flex; align-items: center; gap: 6px; padding: 10px 12px; border-bottom: 1px solid rgba(0, 0, 0, 0.06); }
.icon { width: 14px; height: 14px; color: #64748b; flex-shrink: 0; }
.name { font-weight: 600; font-size: 12px; color: #1e293b; flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.state-pill { display: inline-flex; align-items: center; gap: 4px; font-size: 10px; font-weight: 600; padding: 2px 6px 2px 5px; border-radius: 10px; border: 1px solid; flex-shrink: 0; }
.state-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.tunnel-info { display: flex; flex-direction: column; gap: 6px; padding: 12px; }
.line { display: flex; align-items: center; gap: 6px; }
.dir-badge { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 4px; flex-shrink: 0; }
.dir-badge.local { background: #dbeafe; color: #1d4ed8; }
.dir-badge.remote { background: #fce7f3; color: #9d174d; }
.addr, .target { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; color: #475569; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:deep(.handle-right) {
  width: 12px !important;
  height: 12px !important;
  right: -6px;
  background: #2563eb !important;
  border: 2px solid white !important;
  box-shadow: 0 0 0 1px rgba(37, 99, 235, 0.2);
  z-index: 4;
}
</style>
