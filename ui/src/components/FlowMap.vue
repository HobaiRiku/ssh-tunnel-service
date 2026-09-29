<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { Remote, TrafficCounters, TunnelStatus } from '@/api/client'
import { formatRate, isIdleRate } from '@/format'
import { useI18n } from '@/i18n'
import { useTrafficStore } from '@/stores/traffic'

// A calm live map: every tunnel is one straight lane between a slim "this
// machine" column on the left and its server's column on the right. Lanes are
// grouped by server, so nothing crosses; text sits at the lane ends, never in
// the middle. Moving dots are real traffic (more and faster when busier),
// flowing from the service towards whoever connected. Stopped tunnels are
// hidden unless asked for, so the map shows what is actually happening.
const props = defineProps<{
  tunnels: TunnelStatus[]
  remotes: Remote[]
}>()

const emit = defineEmits<{
  (e: 'select', name: string): void
}>()

const { t } = useI18n()
const traffic = useTrafficStore()

const prefsKey = 'ssh-tunnel-service.flowmap'
function loadPrefs(): { collapsed: boolean; showStopped: boolean } {
  try {
    const raw = localStorage.getItem(prefsKey)
    if (raw) return { collapsed: false, showStopped: false, ...(JSON.parse(raw) as object) }
  } catch { /* storage unavailable */ }
  return { collapsed: false, showStopped: false }
}
const prefs = ref(loadPrefs())
watch(prefs, (v) => {
  try { localStorage.setItem(prefsKey, JSON.stringify(v)) } catch { /* storage unavailable */ }
}, { deep: true })

const root = ref<HTMLElement | null>(null)
const width = ref(900)
let observer: ResizeObserver | null = null
const reducedMotion = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches

onMounted(() => {
  if (!root.value) return
  width.value = root.value.clientWidth
  observer = new ResizeObserver(([entry]) => { width.value = entry.contentRect.width })
  observer.observe(root.value)
})
onBeforeUnmount(() => observer?.disconnect())

const LEFT_W = 116
const RIGHT_W = 168
const LANE = 38
const GROUP_GAP = 12
const PAD = 12

type Tone = 'local' | 'remote' | 'error' | 'stopped'

interface Lane {
  tunnel: TunnelStatus
  id: string
  y: number
  x1: number
  x2: number
  tone: Tone
  counters?: TrafficCounters
  rate: string
  forward: boolean
  down: { dur: number; count: number } | null
  up: { dur: number; count: number } | null
}

function dots(rate: number): { dur: number; count: number } | null {
  if (reducedMotion || isIdleRate(rate)) return null
  const level = Math.min(4, Math.max(1, Math.floor(Math.log10(rate)) - 1))
  return { count: level, dur: 2.8 - level * 0.4 }
}

const stoppedCount = computed(() => props.tunnels.filter((x) => x.state === 'stopped').length)
const running = computed(() => props.tunnels.filter((x) => x.state === 'running'))
const visible = computed(() => props.tunnels.filter((x) => prefs.value.showStopped || x.state !== 'stopped'))

const layout = computed(() => {
  const W = Math.max(width.value, 560)
  const remoteByName = new Map(props.remotes.map((r) => [r.name, r]))
  const rank: Record<TunnelStatus['state'], number> = { running: 0, error: 1, stopped: 2 }
  const groups = [...new Set(visible.value.map((x) => x.remote))]
    .sort((a, b) => a.localeCompare(b))
    .map((name) => ({
      name,
      host: remoteByName.get(name)?.host ?? '',
      tunnels: visible.value.filter((x) => x.remote === name)
        .sort((a, b) => rank[a.state] - rank[b.state] || a.name.localeCompare(b.name)),
    }))

  const x1 = PAD + LEFT_W
  const x2 = W - PAD - RIGHT_W
  const lanes: Lane[] = []
  const servers: { name: string; host: string; y: number; h: number; down: number }[] = []
  let y = PAD
  groups.forEach((g, gi) => {
    if (gi > 0) y += GROUP_GAP
    const top = y
    let down = 0
    for (const tun of g.tunnels) {
      const counters = traffic.tunnels[tun.name]
      const isRunning = tun.state === 'running'
      const tone: Tone = tun.state === 'error' ? 'error' : !isRunning ? 'stopped' : tun.direction === '-L' ? 'local' : 'remote'
      const total = (counters?.down_rate ?? 0) + (counters?.up_rate ?? 0)
      down += counters?.down_rate ?? 0
      lanes.push({
        tunnel: tun,
        id: `lane-${lanes.length}`,
        y: y + LANE / 2,
        x1,
        x2,
        tone,
        counters,
        rate: isRunning && !tun.direct && !isIdleRate(total) ? formatRate(total) : '',
        // -L: the service sits behind the server, so replies flow right→left.
        forward: tun.direction === '-R',
        down: isRunning && counters ? dots(counters.down_rate) : null,
        up: isRunning && counters ? dots(counters.up_rate) : null,
      })
      y += LANE
    }
    servers.push({ name: g.name, host: g.host, y: top, h: y - top, down })
  })
  return { W, H: y + PAD, lanes, servers, left: { x: PAD, y: PAD, h: Math.max(y - PAD, LANE) }, x2 }
})

function statusLabel(l: Lane): string {
  if (l.tone === 'error') return t('flow.failed')
  if (l.tone === 'stopped') return t('overview.statusStopped')
  if (l.tunnel.direct) return t('traffic.directShort')
  return l.rate || t('flow.idle')
}
</script>

<template>
  <section ref="root" class="flow-map" :class="{ collapsed: prefs.collapsed }">
    <header class="bar">
      <span class="title">{{ t('flow.title') }}</span>
      <span class="summary">
        {{ t('flow.summary', { n: running.length }) }}
        <span class="rate">↓ {{ formatRate(traffic.total.down_rate) }} · ↑ {{ formatRate(traffic.total.up_rate) }}</span>
      </span>
      <span class="spacer" />
      <label v-if="stoppedCount && !prefs.collapsed" class="toggle">
        <input v-model="prefs.showStopped" type="checkbox" />
        {{ t('flow.showStopped', { n: stoppedCount }) }}
      </label>
      <button type="button" class="collapse" :aria-expanded="!prefs.collapsed" @click="prefs.collapsed = !prefs.collapsed">
        {{ prefs.collapsed ? t('flow.expand') : t('flow.collapse') }}
      </button>
    </header>

    <template v-if="!prefs.collapsed">
      <p v-if="layout.lanes.length === 0" class="empty">{{ t('flow.nothingRunning') }}</p>
      <svg v-else :viewBox="`0 0 ${layout.W} ${layout.H}`" :height="layout.H" role="img" :aria-label="t('flow.title')">
        <!-- this machine -->
        <rect class="col me" :x="layout.left.x" :y="layout.left.y" :width="LEFT_W" :height="layout.left.h" rx="10" />
        <text class="col-title" :x="layout.left.x + 12" :y="layout.left.y + 20">{{ t('flow.thisMachine') }}</text>

        <!-- servers -->
        <g v-for="srv in layout.servers" :key="srv.name">
          <rect class="col" :x="layout.x2" :y="srv.y" :width="RIGHT_W" :height="srv.h" rx="10" />
          <text class="col-title" :x="layout.x2 + 12" :y="srv.y + 20">{{ srv.name }}</text>
          <text v-if="srv.h >= LANE * 1.5" class="col-sub" :x="layout.x2 + 12" :y="srv.y + 35">{{ srv.host }}</text>
        </g>

        <!-- lanes -->
        <g
          v-for="l in layout.lanes"
          :key="l.tunnel.name"
          class="lane"
          :class="`tone-${l.tone}`"
          tabindex="0"
          role="button"
          :aria-label="l.tunnel.name"
          @click="emit('select', l.tunnel.name)"
          @keydown.enter="emit('select', l.tunnel.name)"
        >
          <rect class="hit" :x="l.x1" :y="l.y - LANE / 2" :width="l.x2 - l.x1" :height="LANE" />
          <path :id="l.id" class="track" :d="`M${l.x1},${l.y + 6} H${l.x2}`" />
          <circle class="port" :cx="l.x1" :cy="l.y + 6" r="3" />
          <circle class="port" :cx="l.x2" :cy="l.y + 6" r="3" />
          <text class="name" :x="l.x1 + 14" :y="l.y - 2">{{ l.tunnel.name }}<tspan class="route" dx="10">{{ l.tunnel.bind_address }}:{{ l.tunnel.bind_port }} → {{ l.tunnel.target_host }}:{{ l.tunnel.target_port }}</tspan></text>
          <text class="status" :x="l.x2 - 12" :y="l.y - 2" text-anchor="end">{{ statusLabel(l) }}</text>
          <template v-if="l.down">
            <circle v-for="k in l.down.count" :key="`d${k}`" r="3" class="dot" :cy="0">
              <animateMotion
                :dur="`${l.down.dur}s`"
                :begin="`${-(k - 1) * l.down.dur / l.down.count}s`"
                repeatCount="indefinite"
                :keyPoints="l.forward ? '0;1' : '1;0'"
                keyTimes="0;1"
                calcMode="linear"
              ><mpath :href="`#${l.id}`" /></animateMotion>
            </circle>
          </template>
          <template v-if="l.up">
            <circle v-for="k in l.up.count" :key="`u${k}`" r="2" class="dot up">
              <animateMotion
                :dur="`${l.up.dur}s`"
                :begin="`${-(k - 1) * l.up.dur / l.up.count}s`"
                repeatCount="indefinite"
                :keyPoints="l.forward ? '1;0' : '0;1'"
                keyTimes="0;1"
                calcMode="linear"
              ><mpath :href="`#${l.id}`" /></animateMotion>
            </circle>
          </template>
        </g>
      </svg>
    </template>
  </section>
</template>

<style scoped>
.flow-map {
  width: 100%;
  border-radius: 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  overflow: hidden;
}
.bar { display: flex; align-items: center; gap: 12px; padding: 10px 14px; font-size: 12.5px; }
.flow-map:not(.collapsed) .bar { border-bottom: 1px solid var(--color-divider-soft); }
.title { font-weight: 650; color: var(--color-text); }
.summary { color: var(--color-text-tertiary); display: inline-flex; gap: 10px; }
.summary .rate { font-variant-numeric: tabular-nums; color: var(--color-text-secondary); }
.spacer { flex: 1; }
.toggle { display: inline-flex; align-items: center; gap: 6px; color: var(--color-text-tertiary); cursor: pointer; user-select: none; }
.collapse { font: inherit; font-size: 12px; border: 0; background: none; color: var(--color-accent); cursor: pointer; padding: 2px 4px; }
.empty { padding: 18px 14px; font-size: 12.5px; color: var(--color-text-muted); }

svg { display: block; width: 100%; }
.col { fill: var(--color-surface-alt); stroke: var(--color-border); stroke-width: 1; }
.col.me { fill: color-mix(in srgb, var(--color-accent) 8%, var(--color-surface)); stroke: color-mix(in srgb, var(--color-accent) 45%, transparent); }
.col-title { font-size: 12.5px; font-weight: 650; fill: var(--color-text); }
.col-sub { font-size: 10.5px; fill: var(--color-text-muted); font-family: var(--font-mono); }

.lane { cursor: pointer; outline: none; --tone: var(--color-state-stopped-dot); }
.lane.tone-local { --tone: var(--color-dir-local); }
.lane.tone-remote { --tone: var(--color-dir-remote); }
.lane.tone-error { --tone: var(--color-state-error-border); }
.hit { fill: transparent; }
.lane:hover .hit, .lane:focus-visible .hit { fill: var(--color-surface-hover); }
.track { fill: none; stroke: var(--tone); stroke-width: 2; stroke-opacity: 0.45; stroke-linecap: round; }
.tone-error .track { stroke-dasharray: 4 6; stroke-opacity: 0.4; }
.tone-stopped .track { stroke-dasharray: 2 5; stroke-opacity: 0.5; }
.port { fill: var(--color-surface); stroke: var(--tone); stroke-width: 1.5; }
.name { font-size: 12px; font-weight: 600; fill: var(--color-text); }
.route { font-size: 11px; font-weight: 400; fill: var(--color-text-muted); font-family: var(--font-mono); }
.tone-stopped .name { fill: var(--color-text-muted); font-weight: 500; }
.status { font-size: 11px; fill: var(--color-text-tertiary); font-variant-numeric: tabular-nums; }
.tone-local .status, .tone-remote .status { fill: var(--color-text-secondary); }
.tone-error .status { fill: var(--color-danger); }
.dot { fill: var(--tone); }
.dot.up { fill-opacity: 0.5; }
</style>
