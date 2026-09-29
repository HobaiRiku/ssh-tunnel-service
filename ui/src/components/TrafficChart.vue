<script setup lang="ts">
import { computed } from 'vue'
import type { TrafficPoint } from '@/api/client'
import { formatRate } from '@/format'

// The detail chart: the last `span` seconds of one tunnel, "down" as a filled
// area and "up" as a line, with a rate scale and a time axis.
const props = withDefaults(defineProps<{
  points: TrafficPoint[]
  span?: number
  height?: number
  color?: string
  nowLabel: string
  agoLabel: string
}>(), {
  span: 300,
  height: 150,
  color: 'var(--color-accent)',
})

const W = 440
const PAD_L = 4
const PAD_R = 4
const PAD_T = 8
const PAD_B = 18
const MIN_SCALE = 4096

// Round the scale up to 1/2/5 × 10^n so gridline labels read cleanly.
function niceCeil(v: number): number {
  const exp = Math.pow(10, Math.floor(Math.log10(v)))
  const f = v / exp
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10
  return nice * exp
}

const chart = computed(() => {
  const H = props.height
  const plotH = H - PAD_T - PAD_B
  const pts = props.points
  const end = pts.length ? pts[pts.length - 1].t : Date.now()
  const start = end - props.span * 1000
  const visible = pts.filter((p) => p.t >= start)
  const max = niceCeil(Math.max(MIN_SCALE, ...visible.map((p) => Math.max(p.up, p.down))))
  const x = (t: number) => PAD_L + ((t - start) / (end - start)) * (W - PAD_L - PAD_R)
  const y = (v: number) => PAD_T + plotH - (v / max) * plotH
  const line = (key: 'up' | 'down') => visible.map((p, i) => `${i ? 'L' : 'M'}${x(p.t).toFixed(1)},${y(p[key]).toFixed(1)}`).join('')
  const down = line('down')
  const area = visible.length > 1 ? `${down}L${x(visible[visible.length - 1].t).toFixed(1)},${PAD_T + plotH}L${x(visible[0].t).toFixed(1)},${PAD_T + plotH}Z` : ''
  const grid = [1, 0.5, 0].map((f) => ({ y: y(max * f), label: f ? formatRate(max * f) : '' }))
  return { H, area, down, up: line('up'), grid, base: PAD_T + plotH }
})
</script>

<template>
  <div class="chart">
    <svg :viewBox="`0 0 ${W} ${chart.H}`" :style="{ height: `${chart.H}px`, color }" preserveAspectRatio="none" aria-hidden="true">
      <line v-for="g in chart.grid" :key="g.y" class="grid" :x1="0" :x2="W" :y1="g.y" :y2="g.y" />
      <path v-if="chart.area" class="area" :d="chart.area" />
      <path class="down" :d="chart.down" />
      <path class="up" :d="chart.up" />
    </svg>
    <span v-for="g in chart.grid.filter((x) => x.label)" :key="g.label" class="ylabel" :style="{ top: `${g.y - 14}px` }">{{ g.label }}</span>
    <div class="xaxis"><span>{{ agoLabel }}</span><span>{{ nowLabel }}</span></div>
  </div>
</template>

<style scoped>
.chart { position: relative; }
svg { display: block; width: 100%; }
.grid { stroke: var(--color-border); stroke-width: 1; stroke-dasharray: 2 3; vector-effect: non-scaling-stroke; }
.area { fill: currentColor; fill-opacity: 0.16; }
.down { fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linejoin: round; vector-effect: non-scaling-stroke; }
.up { fill: none; stroke: currentColor; stroke-opacity: 0.6; stroke-width: 1.3; stroke-dasharray: 4 3; vector-effect: non-scaling-stroke; }
.ylabel { position: absolute; left: 4px; font-size: 10.5px; color: var(--color-text-muted); font-variant-numeric: tabular-nums; pointer-events: none; }
.xaxis { position: absolute; left: 0; right: 0; bottom: 0; display: flex; justify-content: space-between; font-size: 10.5px; color: var(--color-text-muted); padding: 0 4px; }
</style>
