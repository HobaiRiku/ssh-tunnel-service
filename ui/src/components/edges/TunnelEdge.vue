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
const bindPortText = computed(() => (props.data?.bindPort == null ? '' : String(props.data.bindPort)))
const targetPortText = computed(() => (props.data?.targetPort == null ? '' : String(props.data.targetPort)))
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
  align-items: baseline;
  gap: 4px;
  padding: 1px 6px;
  background: rgba(255, 255, 255, 0.85);
  color: #475569;
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
  pointer-events: none;
  z-index: 6;
  border-radius: 4px;
}

.tunnel-edge-port {
  color: #0f172a;
}

.tunnel-edge-icon {
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
}

.forward .tunnel-edge-icon { color: #1d4ed8; }
.reverse .tunnel-edge-icon { color: #a21caf; }

.selected .tunnel-edge-icon { color: #2563eb; }
.selected .tunnel-edge-port { color: #1d4ed8; }
</style>
