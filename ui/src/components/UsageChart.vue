<script setup lang="ts">
import { computed } from 'vue'
import type { UsageHistory } from '@/api/client'
import { formatBytes } from '@/format'

// Long-term traffic as bars: the service's minute/hour/day buckets are folded
// into a readable number of bars — half-hours over 24 hours, days over 30 days,
// months over a year — each stacked "down" (solid) over "up" (light).
const props = withDefaults(defineProps<{
  history: UsageHistory | null
  color?: string
  height?: number
  locale: string
}>(), {
  color: 'var(--color-accent)',
  height: 150,
})

interface Bin { start: number; end: number; up: number; down: number; label: string }

function bins(h: UsageHistory): Bin[] {
  const out: Bin[] = []
  const fmt = (d: Date, opts: Intl.DateTimeFormatOptions) => d.toLocaleString(props.locale, opts)
  if (h.range === '24h') {
    const step = 30 * 60
    const end = Math.ceil(h.to / step) * step
    for (let s = end - 48 * step; s < end; s += step) {
      out.push({ start: s, end: s + step, up: 0, down: 0, label: fmt(new Date(s * 1000), { hour: '2-digit', minute: '2-digit' }) })
    }
  } else if (h.range === '30d') {
    const today = new Date(h.to * 1000)
    today.setHours(0, 0, 0, 0)
    for (let i = 29; i >= 0; i--) {
      const d = new Date(today)
      d.setDate(d.getDate() - i)
      const next = new Date(d)
      next.setDate(next.getDate() + 1)
      out.push({ start: d.getTime() / 1000, end: next.getTime() / 1000, up: 0, down: 0, label: fmt(d, { month: 'numeric', day: 'numeric' }) })
    }
  } else {
    const now = new Date(h.to * 1000)
    for (let i = 11; i >= 0; i--) {
      const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
      const next = new Date(now.getFullYear(), now.getMonth() - i + 1, 1)
      out.push({ start: d.getTime() / 1000, end: next.getTime() / 1000, up: 0, down: 0, label: fmt(d, { year: '2-digit', month: 'short' }) })
    }
  }
  for (const [t, up, down] of h.points) {
    const bin = out.find((b) => t >= b.start && t < b.end)
    if (bin) {
      bin.up += up
      bin.down += down
    }
  }
  return out
}

const W = 440
const PAD_T = 8
const PAD_B = 18

const chart = computed(() => {
  const H = props.height
  const plotH = H - PAD_T - PAD_B
  const list = props.history ? bins(props.history) : []
  const max = Math.max(1, ...list.map((b) => b.up + b.down))
  const slot = W / Math.max(list.length, 1)
  const gap = Math.min(3, slot * 0.25)
  const bars = list.map((b, i) => {
    const x = i * slot + gap / 2
    const w = Math.max(1, slot - gap)
    const hDown = (b.down / max) * plotH
    const hUp = (b.up / max) * plotH
    const base = PAD_T + plotH
    return {
      x, w,
      down: { y: base - hDown, h: hDown },
      up: { y: base - hDown - hUp, h: hUp },
      title: `${b.label}  ↓ ${formatBytes(b.down)}  ↑ ${formatBytes(b.up)}`,
    }
  })
  const totals = list.reduce((acc, b) => ({ up: acc.up + b.up, down: acc.down + b.down }), { up: 0, down: 0 })
  const ticks = list.length ? [list[0].label, list[Math.floor(list.length / 2)].label, list[list.length - 1].label] : []
  return { H, bars, maxLabel: formatBytes(max), base: PAD_T + plotH, totals, ticks, empty: totals.up + totals.down === 0 }
})

</script>

<template>
  <div class="usage-chart">
    <svg :viewBox="`0 0 ${W} ${chart.H}`" :style="{ height: `${chart.H}px`, color }" preserveAspectRatio="none" role="img">
      <line class="grid" x1="0" :x2="W" :y1="PAD_T" :y2="PAD_T" />
      <line class="grid base" x1="0" :x2="W" :y1="chart.base" :y2="chart.base" />
      <g v-for="(b, i) in chart.bars" :key="i" class="bar">
        <title>{{ b.title }}</title>
        <rect class="hit" :x="b.x" :y="PAD_T" :width="b.w" :height="chart.base - PAD_T" />
        <rect v-if="b.up.h > 0" class="up" :x="b.x" :y="b.up.y" :width="b.w" :height="b.up.h" />
        <rect v-if="b.down.h > 0" class="down" :x="b.x" :y="b.down.y" :width="b.w" :height="b.down.h" />
      </g>
    </svg>
    <span v-if="!chart.empty" class="ymax">{{ chart.maxLabel }}</span>
    <div class="xaxis"><span v-for="(l, i) in chart.ticks" :key="i">{{ l }}</span></div>
  </div>
</template>

<style scoped>
.usage-chart { position: relative; }
svg { display: block; width: 100%; }
.grid { stroke: var(--color-border); stroke-width: 1; stroke-dasharray: 2 3; vector-effect: non-scaling-stroke; }
.grid.base { stroke-dasharray: none; }
.hit { fill: transparent; }
.bar:hover .hit { fill: var(--color-surface-hover); }
.down { fill: currentColor; fill-opacity: 0.85; }
.up { fill: currentColor; fill-opacity: 0.35; }
.ymax { position: absolute; left: 4px; top: -6px; font-size: 10.5px; color: var(--color-text-muted); font-variant-numeric: tabular-nums; pointer-events: none; }
.xaxis { position: absolute; left: 0; right: 0; bottom: 0; display: flex; justify-content: space-between; font-size: 10.5px; color: var(--color-text-muted); padding: 0 2px; }
</style>
