<script setup lang="ts">
import { computed } from 'vue'
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath } from '@vue-flow/core'
import type { EdgeProps } from '@vue-flow/core'

type TunnelEdgeData = {
  remotePort?: number
  localPort?: number
  selected?: boolean
  reversed?: boolean
}

const props = defineProps<EdgeProps<TunnelEdgeData>>()

const edgePath = computed(() =>
  getSmoothStepPath({
    sourceX: props.sourceX,
    sourceY: props.sourceY,
    sourcePosition: props.sourcePosition,
    targetX: props.targetX,
    targetY: props.targetY,
    targetPosition: props.targetPosition,
    borderRadius: 20,
    offset: 30,
  }),
)

const labelPositionStyle = computed(() => ({
  transform: `translate(-50%, -50%) translate(${edgePath.value[1]}px, ${edgePath.value[2]}px)`,
}))

const remotePortText = computed(() => (props.data?.remotePort == null ? '' : String(props.data.remotePort)))
const localPortText = computed(() => (props.data?.localPort == null ? '' : String(props.data.localPort)))
</script>

<template>
  <BaseEdge
    :id="id"
    :path="edgePath[0]"
    :style="style"
    :marker-start="markerStart"
    :marker-end="markerEnd"
  />

  <EdgeLabelRenderer>
    <div
      class="tunnel-edge-label nodrag nopan"
      :class="{ selected: data?.selected }"
      :style="labelPositionStyle"
    >
      <span class="tunnel-edge-port">{{ remotePortText }}</span>
      <span class="tunnel-edge-icon" aria-hidden="true" :style="{ color: data?.reversed ? 'var(--color-tag-pink-text)' : 'var(--color-tag-blue-text)' }">{{ data?.reversed ? '←' : '→' }}</span>
      <span class="tunnel-edge-port">{{ localPortText }}</span>
    </div>
  </EdgeLabelRenderer>
</template>

<style scoped>
.tunnel-edge-label {
  position: absolute;
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  padding: 1px 6px;
  background: var(--color-overlay);
  color: var(--color-text-secondary);
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
  pointer-events: none;
  z-index: 6;
  border-radius: 4px;
}

.tunnel-edge-port {
  color: var(--color-text);
}

.tunnel-edge-icon {
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
}

.selected .tunnel-edge-port { color: var(--color-accent-strong); }
</style>
