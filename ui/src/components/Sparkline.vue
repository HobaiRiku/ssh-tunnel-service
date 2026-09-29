<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficPoint } from '@/api/client'

// A tiny live traffic chart: "down" (from the service) as a soft filled area,
// "up" (to the service) as a thin line on the same scale. It draws the last
// `span` seconds against the newest sample, so gaps show as flat stretches
// instead of being squeezed out.
const props = withDefaults(defineProps<{
  points: TrafficPoint[]
  span?: number
  color?: string
  height?: number
}>(), {
  span: 120,
  color: 'var(--color-accent)',
  height: 36,
})

const W = 300
// Floor of the vertical scale, so a trickle of keep-alive bytes does not look
// like a saturated link.
const MIN_SCALE = 2048

const view = computed(() => {
  const pts = props.points
  const H = props.height
  if (pts.length === 0) return { area: '', up: '', empty: true }
  const end = pts[pts.length - 1].t
  const start = end - props.span * 1000
  const visible = pts.filter((p) => p.t >= start)
  const max = Math.max(MIN_SCALE, ...visible.map((p) => Math.max(p.up, p.down)))
  const x = (t: number) => ((t - start) / (end - start || 1)) * W
  const y = (v: number) => H - 1 - (v / max) * (H - 3)
  const line = (key: 'up' | 'down') => visible.map((p, i) => `${i === 0 ? 'M' : 'L'}${x(p.t).toFixed(1)},${y(p[key]).toFixed(1)}`).join('')
  const down = line('down')
  const first = visible[0]
  const area = visible.length > 1 ? `${down}L${x(end).toFixed(1)},${H}L${x(first.t).toFixed(1)},${H}Z` : ''
  const quiet = visible.every((p) => p.up < 1 && p.down < 1)
  return { area, up: line('up'), empty: quiet }
})
</script>

<template>
  <svg class="sparkline" :viewBox="`0 0 ${W} ${height}`" :style="{ height: `${height}px`, color }" preserveAspectRatio="none" aria-hidden="true">
    <line class="baseline" x1="0" :y1="height - 0.5" :x2="W" :y2="height - 0.5" />
    <template v-if="!view.empty">
      <path class="down" :d="view.area" />
      <path class="up" :d="view.up" />
    </template>
  </svg>
</template>

<style scoped>
.sparkline { display: block; width: 100%; overflow: visible; }
.baseline { stroke: var(--color-border); stroke-width: 1; vector-effect: non-scaling-stroke; }
.down { fill: currentColor; fill-opacity: 0.22; stroke: currentColor; stroke-width: 1.5; stroke-linejoin: round; vector-effect: non-scaling-stroke; }
.up { fill: none; stroke: currentColor; stroke-opacity: 0.7; stroke-width: 1.25; stroke-dasharray: 3 2; vector-effect: non-scaling-stroke; }
</style>
