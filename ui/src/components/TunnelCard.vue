<script setup lang="ts">
import { computed, h } from 'vue'
import { NButton, NDropdown, NSwitch, useMessage } from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import type { Remote, TrafficCounters, TrafficPoint, TunnelStatus } from '@/api/client'
import Sparkline from '@/components/Sparkline.vue'
import TunnelProblem from '@/components/TunnelProblem.vue'
import { copyText } from '@/clipboard'
import { formatBytes, formatRate, isIdleRate } from '@/format'
import { useI18n } from '@/i18n'
import { useTrafficStore } from '@/stores/traffic'
import { connectAddress, useTunnelText, type TunnelCardAction } from '@/tunnelText'

const props = defineProps<{
  tunnel: TunnelStatus
  remote?: Remote
  counters?: TrafficCounters
  points: TrafficPoint[]
  busy?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle', tunnel: TunnelStatus, on: boolean): void
  (e: 'action', action: TunnelCardAction, tunnel: TunnelStatus): void
}>()

const { t } = useI18n()
const message = useMessage()
const text = useTunnelText()

const ends = computed(() => text.ends(props.tunnel))
const running = computed(() => props.tunnel.state === 'running')
const metered = computed(() => !props.tunnel.direct)
// A tunnel that is down and never moved a byte has nothing to chart.
const showLive = computed(() => {
  if (running.value) return true
  const c = counters.value
  return metered.value && !!c && c.down_bytes + c.up_bytes > 0
})
const counters = computed(() => props.counters ?? props.tunnel.traffic)
const address = computed(() => connectAddress(props.tunnel, props.remote))
// Today's traffic from the persisted book (the card's headline total).
const trafficStore = useTrafficStore()
const todayBytes = computed(() => {
  const u = trafficStore.usage?.tunnels[props.tunnel.name]?.today
  return (u?.down_bytes ?? 0) + (u?.up_bytes ?? 0)
})
// Moving traffic animates the route arrow; an idle tunnel stays still.
const flowing = computed(() => running.value && !!counters.value && !(isIdleRate(counters.value.down_rate) && isIdleRate(counters.value.up_rate)))

// Running tunnels take their direction's colour; being down always wins.
const tone = computed(() => {
  if (props.tunnel.state === 'error') return 'error'
  if (!running.value) return 'stopped'
  return props.tunnel.direction === '-L' ? 'local' : 'remote'
})

const statusText = computed(() => {
  const tun = props.tunnel
  if (tun.state === 'error') return tun.auto_start ? t('overview.statusRetrying') : t('overview.statusError')
  if (tun.state === 'stopped') return t('overview.statusStopped')
  return tun.auto_start ? t('overview.statusKeptAlive') : t('overview.statusRunning')
})

const menu = computed<DropdownOption[]>(() => [
  { key: 'details', label: t('detail.open') },
  ...(props.tunnel.state !== 'stopped' ? [{ key: 'restart', label: t('common.restart') }] : []),
  { key: 'edit', label: t('common.edit') },
  { key: 'command', label: t('common.ssh') },
  { key: 'copyName', label: t('common.copyName') },
  { type: 'divider', key: 'd1' },
  { key: 'delete', label: () => h('span', { style: 'color: var(--color-danger)' }, t('common.delete')) },
])

async function copyAddress() {
  const ok = await copyText(address.value)
  if (ok) message.success(t('overview.addressCopied', { address: address.value }))
  else message.error(t('common.copyFailed'))
}
</script>

<template>
  <article class="tunnel-card" :class="`tone-${tone}`" :data-tunnel="tunnel.name">
    <header class="card-head">
      <span class="dir-badge" :class="tunnel.direction === '-L' ? 'local' : 'remote'" :title="tunnel.direction">
        <svg v-if="tunnel.direction === '-L'" viewBox="0 0 20 20" aria-hidden="true"><path d="M14 4v12M4 10h8m-3-3 3 3-3 3" /></svg>
        <svg v-else viewBox="0 0 20 20" aria-hidden="true"><path d="M6 4v12M16 10H8m3-3-3 3 3 3" /></svg>
      </span>
      <div class="titles" role="button" tabindex="0" @click="emit('action', 'details', tunnel)" @keydown.enter="emit('action', 'details', tunnel)">
        <h3 class="name" :title="tunnel.name">{{ tunnel.name }}</h3>
        <p class="kind">{{ text.kind(tunnel) }}<template v-if="tunnel.description"> · {{ tunnel.description }}</template></p>
      </div>
      <n-switch
        :value="running"
        :loading="busy"
        size="small"
        :aria-label="running ? t('common.stop') : t('common.start')"
        @update:value="(on: boolean) => emit('toggle', tunnel, on)"
      />
    </header>

    <div class="route">
      <div class="end entry">
        <span class="where">{{ ends.entryWhere }}</span>
        <code class="addr">{{ ends.entry }}</code>
      </div>
      <span class="route-line" :class="{ flowing }" aria-hidden="true"><span class="dash" /></span>
      <div class="end service">
        <span class="where">{{ ends.serviceWhere }}</span>
        <code class="addr">{{ ends.service }}</code>
      </div>
    </div>

    <TunnelProblem v-if="tunnel.state === 'error'" :tunnel="tunnel" :remote="remote" @action="(a) => emit('action', a, tunnel)" />

    <div v-if="showLive" class="live" @click="emit('action', 'details', tunnel)">
      <template v-if="metered">
        <Sparkline :points="points" :span="120" :height="34" color="var(--card-tone)" />
        <div class="stats">
          <span class="stat" :class="{ idle: isIdleRate(counters?.down_rate ?? 0) }" :title="t('traffic.downHint')">
            <b>↓</b> {{ formatRate(counters?.down_rate ?? 0) }}
          </span>
          <span class="stat" :class="{ idle: isIdleRate(counters?.up_rate ?? 0) }" :title="t('traffic.upHint')">
            <b>↑</b> {{ formatRate(counters?.up_rate ?? 0) }}
          </span>
          <span v-if="(counters?.active_conns ?? 0) > 0" class="stat conns">{{ t('traffic.conns', { n: counters?.active_conns ?? 0 }) }}</span>
          <span class="stat total" :title="t('traffic.totalHint', { all: formatBytes((counters?.down_bytes ?? 0) + (counters?.up_bytes ?? 0)) })">
            {{ t('traffic.todayShort', { amount: formatBytes(todayBytes) }) }}
          </span>
        </div>
      </template>
      <p v-else class="unmetered">{{ t('traffic.direct') }}</p>
    </div>

    <footer class="card-foot">
      <span class="status"><span class="dot" />{{ statusText }}</span>
      <div class="actions">
        <n-button size="small" tertiary :title="address" @click="copyAddress">{{ t('overview.copyAddress') }}</n-button>
        <n-dropdown trigger="click" :options="menu" placement="bottom-end" @select="(key: TunnelCardAction) => emit('action', key, tunnel)">
          <n-button size="small" quaternary :aria-label="t('overview.more')">
            <svg class="more-icon" viewBox="0 0 20 20" aria-hidden="true"><circle cx="4" cy="10" r="1.6" /><circle cx="10" cy="10" r="1.6" /><circle cx="16" cy="10" r="1.6" /></svg>
          </n-button>
        </n-dropdown>
      </div>
    </footer>
  </article>
</template>

<style scoped>
.tunnel-card {
  --card-tone: var(--color-state-stopped-dot);
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px 12px;
  border-radius: 14px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  box-shadow: 0 1px 2px var(--color-shadow-soft);
  position: relative;
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
  min-width: 0;
}
.tunnel-card::before {
  content: '';
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--card-tone);
  opacity: 0.9;
}
.tunnel-card.flash { animation: flash 1.2s ease-out; }
@keyframes flash {
  0%, 30% { box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-accent) 55%, transparent); }
  100% { box-shadow: 0 1px 2px var(--color-shadow-soft); }
}
.tunnel-card:hover { box-shadow: 0 6px 20px var(--color-shadow-soft); border-color: var(--color-border-strong); }
.tone-local { --card-tone: var(--color-dir-local); }
.tone-remote { --card-tone: var(--color-dir-remote); }
.tone-error { --card-tone: var(--color-state-error-border); }

.card-head { display: flex; align-items: center; gap: 10px; min-width: 0; }
.dir-badge {
  width: 32px; height: 32px; border-radius: 10px; flex-shrink: 0;
  display: inline-flex; align-items: center; justify-content: center;
}
.dir-badge svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.dir-badge.local { background: var(--color-dir-local-soft); color: var(--color-dir-local-text); }
.dir-badge.remote { background: var(--color-dir-remote-soft); color: var(--color-dir-remote-text); }
.titles { flex: 1; min-width: 0; cursor: pointer; border-radius: 6px; outline: none; }
.titles:hover .name, .titles:focus-visible .name { color: var(--color-accent); }
.name { font-size: 14.5px; font-weight: 650; letter-spacing: -0.01em; color: var(--color-text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.kind { font-size: 12px; color: var(--color-text-tertiary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-top: 1px; }

/* Addresses keep their natural width and the arrow takes what is left; only
   when a card is truly too narrow do they shrink and ellipsize. */
.route { display: grid; grid-template-columns: minmax(0, max-content) minmax(20px, 1fr) minmax(0, max-content); align-items: end; gap: 10px; }
.route-line { margin-bottom: 7px; }
.end { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.end.service { text-align: right; }
.where { font-size: 11px; color: var(--color-text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.addr { font-family: var(--font-mono); font-size: 12.5px; color: var(--color-text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.route-line { position: relative; height: 2px; background: var(--color-border); border-radius: 2px; overflow: hidden; }
.route-line::after {
  content: ''; position: absolute; right: -1px; top: -3px;
  border: 4px solid transparent; border-left: 5px solid var(--color-border-strong); border-right: 0;
}
.route-line .dash {
  position: absolute; inset: 0;
  background: repeating-linear-gradient(90deg, var(--card-tone) 0 6px, transparent 6px 12px);
  opacity: 0;
}
.route-line.flowing .dash { opacity: 1; animation: flow 0.8s linear infinite; }
@keyframes flow { from { background-position: 0 0; } to { background-position: 12px 0; } }


.live { cursor: pointer; display: flex; flex-direction: column; justify-content: flex-end; gap: 6px; flex: 1; }
.stats { display: flex; gap: 12px; white-space: nowrap; font-size: 12px; font-variant-numeric: tabular-nums; color: var(--color-text-secondary); }
.stat b { font-weight: 700; color: var(--card-tone); }
.stat.idle { color: var(--color-text-muted); }
.stat.conns { color: var(--color-text-tertiary); }
.stat.total { margin-left: auto; color: var(--color-text-muted); }
.unmetered { font-size: 12px; color: var(--color-text-muted); padding: 8px 0; }

.card-foot { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-top: 1px solid var(--color-divider-soft); padding-top: 10px; }
.status { display: inline-flex; align-items: center; gap: 7px; font-size: 12px; color: var(--color-text-secondary); min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--card-tone); flex-shrink: 0; }
.tone-local .dot, .tone-remote .dot { box-shadow: 0 0 0 0 var(--card-tone); animation: pulse 2s ease-out infinite; }
@keyframes pulse {
  0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--card-tone) 55%, transparent); }
  100% { box-shadow: 0 0 0 7px transparent; }
}
.actions { display: flex; align-items: center; gap: 2px; flex-shrink: 0; }
.more-icon { width: 16px; height: 16px; fill: currentColor; }

@media (prefers-reduced-motion: reduce) {
  .route-line.flowing .dash, .tone-local .dot, .tone-remote .dot { animation: none; }
}
</style>
