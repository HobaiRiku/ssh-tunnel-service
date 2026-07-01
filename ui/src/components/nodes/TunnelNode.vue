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

const arrowColor = computed(() => (props.data.direction === '-R' ? 'var(--color-tag-pink-text)' : 'var(--color-tag-blue-text)'))

const stateStyle = computed(() => {
  switch (props.data.state) {
    case 'running':
      return { bg: 'var(--color-state-running-bg)', border: 'var(--color-state-running-border)', dot: 'var(--color-state-running-border)', label: t('common.running') }
    case 'error':
      return { bg: 'var(--color-state-error-bg)', border: 'var(--color-state-error-border)', dot: 'var(--color-state-error-border)', label: t('common.error') }
    default:
      return { bg: 'var(--color-state-stopped-bg)', border: 'var(--color-state-stopped-border)', dot: 'var(--color-state-stopped-dot)', label: t('common.stopped') }
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
  box-shadow: 0 1px 4px var(--color-shadow-soft);
  cursor: pointer;
  transition: box-shadow 0.15s ease, transform 0.15s ease, border-color 0.15s ease;
}

.tunnel-node:hover,
.tunnel-node.selected {
  box-shadow: 0 8px 18px var(--color-shadow-medium);
  transform: translateY(-1px);
}

.tunnel-node.selected { border-color: var(--color-accent) !important; }
.tunnel-node:focus-visible { outline: 2px solid var(--color-accent); outline-offset: 2px; }
.tunnel-top { display: flex; align-items: center; gap: 6px; padding: 10px 12px; border-bottom: 1px solid var(--color-divider-soft); }
.icon { width: 14px; height: 14px; color: var(--color-text-tertiary); flex-shrink: 0; }
.name { font-weight: 600; font-size: 12px; color: var(--color-text); flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.state-pill { display: inline-flex; align-items: center; gap: 4px; font-size: 10px; font-weight: 600; padding: 2px 6px 2px 5px; border-radius: 10px; border: 1px solid; flex-shrink: 0; }
.state-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.tunnel-info { display: flex; flex-direction: column; gap: 6px; padding: 12px; }
.line { display: flex; align-items: center; gap: 6px; }
.endpoint-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--color-text-tertiary);
  flex-shrink: 0;
}
.endpoint-label--local { color: var(--color-node-target-border); }
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
.dir-badge.local { background: var(--color-tag-blue-bg); color: var(--color-tag-blue-text); }
.dir-badge.remote { background: var(--color-tag-pink-bg); color: var(--color-tag-pink-text); }
.addr { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; color: var(--color-text-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
:deep(.handle-right) {
  width: 10px !important;
  height: 10px !important;
  right: -5px;
  background: var(--color-accent) !important;
  border: 2px solid var(--color-surface) !important;
  z-index: 4;
  animation: tunnel-handle-pulse 1.8s ease-out infinite;
}

@keyframes tunnel-handle-pulse {
  0% { box-shadow: 0 0 0 0 rgba(37, 99, 235, 0.55); }
  70% { box-shadow: 0 0 0 10px rgba(37, 99, 235, 0); }
  100% { box-shadow: 0 0 0 0 rgba(37, 99, 235, 0); }
}
</style>
