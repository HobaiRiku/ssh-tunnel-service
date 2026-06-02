<script setup lang="ts">
import { computed } from 'vue'
import { Handle, Position } from '@vue-flow/core'
import { useI18n } from '@/i18n'

const props = defineProps<{
  data: {
    label: string
    direction: string
    remoteHost: string
    remotePort: number
    localHost: string
    localPort: number
    state: string
    selected?: boolean
    onSelect?: () => void
  }
}>()

const { t } = useI18n()

const arrowColor = computed(() => (props.data.direction === '-R' ? '#9d174d' : '#1d4ed8'))

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
      <span class="name">{{ data.label }}</span>
      <span class="state-pill" :style="{ background: stateStyle.bg, color: stateStyle.dot, borderColor: stateStyle.border }">
        <span class="state-dot" :style="{ background: stateStyle.dot }"></span>
        {{ stateStyle.label }}
      </span>
    </div>
    <div class="tunnel-info">
      <div class="line">
        <span class="dir-badge" :class="data.direction === '-L' ? 'local' : 'remote'">{{ data.direction }}</span>
        <span class="endpoint-label">{{ t('topology.remote') }}</span>
        <span class="addr">{{ data.remoteHost }}:{{ data.remotePort }}</span>
      </div>
      <div class="target">
        <span class="target-arrow" aria-hidden="true" :style="{ color: arrowColor }">{{ data.direction === '-R' ? '←' : '→' }}</span>
        <span class="endpoint-label endpoint-label--local">{{ t('topology.local') }}</span>
        <span class="addr">{{ data.localHost }}:{{ data.localPort }}</span>
      </div>
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
.endpoint-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: #64748b;
  flex-shrink: 0;
}
.endpoint-label--local { color: #7c3aed; }
.target {
  display: flex;
  align-items: center;
  gap: 6px;
}
.target-arrow {
  font-size: 13px;
  font-weight: 700;
  flex-shrink: 0;
}
.dir-badge { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 10px; font-weight: 700; padding: 1px 5px; border-radius: 4px; flex-shrink: 0; }
.dir-badge.local { background: #dbeafe; color: #1d4ed8; }
.dir-badge.remote { background: #fce7f3; color: #9d174d; }
.addr { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; color: #475569; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:deep(.handle-right) {
  width: 10px !important;
  height: 10px !important;
  right: -5px;
  background: #2563eb !important;
  border: 2px solid #fff !important;
  z-index: 4;
  animation: tunnel-handle-pulse 1.8s ease-out infinite;
}

@keyframes tunnel-handle-pulse {
  0% { box-shadow: 0 0 0 0 rgba(37, 99, 235, 0.55); }
  70% { box-shadow: 0 0 0 10px rgba(37, 99, 235, 0); }
  100% { box-shadow: 0 0 0 0 rgba(37, 99, 235, 0); }
}
</style>
