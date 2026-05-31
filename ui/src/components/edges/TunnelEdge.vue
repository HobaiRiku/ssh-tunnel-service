<script setup lang="ts">
import { computed } from 'vue'
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath } from '@vue-flow/core'
import type { EdgeProps } from '@vue-flow/core'

type TunnelEdgeData = {
  direction?: '-L' | '-R'
  bindPort?: number
  targetPort?: number
  selected?: boolean
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

const directionIcon = computed(() => (props.data?.direction === '-R' ? '←' : '→'))
const directionClass = computed(() => (props.data?.direction === '-R' ? 'reverse' : 'forward'))
const bindPortText = computed(() => (props.data?.bindPort == null ? '' : `:${props.data.bindPort}`))
const targetPortText = computed(() => (props.data?.targetPort == null ? '' : `:${props.data.targetPort}`))
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
      :class="[directionClass, { selected: data?.selected }]"
      :style="labelPositionStyle"
    >
      <span class="tunnel-edge-port">{{ bindPortText }}</span>
      <span class="tunnel-edge-icon" aria-hidden="true">{{ directionIcon }}</span>
      <span class="tunnel-edge-port">{{ targetPortText }}</span>
    </div>
  </EdgeLabelRenderer>
</template>

<style scoped>
.tunnel-edge-label {
  position: absolute;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  border-radius: 999px;
  border: 1px solid #cbd5e1;
  background: rgba(255, 255, 255, 0.99);
  color: #334155;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.12);
  font-size: 12px;
  font-weight: 800;
  line-height: 1;
  letter-spacing: 0.02em;
  pointer-events: none;
  z-index: 6;
}

.tunnel-edge-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
  border-radius: 999px;
  background: currentColor;
  color: #fff;
  font-size: 11px;
}

.tunnel-edge-port {
  min-width: 54px;
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(241, 245, 249, 0.9);
  color: #0f172a;
  text-align: center;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
}

.forward {
  border-color: #bfdbfe;
  color: #1d4ed8;
}

.reverse {
  border-color: #f5d0fe;
  color: #a21caf;
}

.selected {
  box-shadow: 0 14px 32px rgba(37, 99, 235, 0.2);
}
</style>
