<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NDrawer, NDrawerContent, NSwitch, useMessage } from 'naive-ui'
import { api, getErrorMessage, type Remote, type TunnelStatus, type UsageHistory, type UsageRange, type UsageTotals } from '@/api/client'
import TrafficChart from '@/components/TrafficChart.vue'
import UsageChart from '@/components/UsageChart.vue'
import TunnelProblem from '@/components/TunnelProblem.vue'
import { copyText } from '@/clipboard'
import { formatBytes, formatRate } from '@/format'
import { useI18n } from '@/i18n'
import { useTrafficStore } from '@/stores/traffic'
import { connectAddress, useTunnelText, type TunnelCardAction } from '@/tunnelText'

// Everything about one tunnel in a side panel: what it connects, whether it
// works (and how to fix it if not), five minutes of traffic, and the rarer
// actions. The tunnel prop is the live store object, so it updates in place.
const props = defineProps<{
  show: boolean
  tunnel: TunnelStatus | null
  remote?: Remote
  busy?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'toggle', tunnel: TunnelStatus, on: boolean): void
  (e: 'action', action: TunnelCardAction, tunnel: TunnelStatus): void
}>()

const { t, locale } = useI18n()
const message = useMessage()
const traffic = useTrafficStore()
const text = useTunnelText()

const width = computed(() => (typeof window !== 'undefined' && window.innerWidth < 640 ? '100%' : 460))
const counters = computed(() => (props.tunnel ? traffic.tunnels[props.tunnel.name] ?? props.tunnel.traffic : undefined))
const points = computed(() => (props.tunnel ? traffic.history[props.tunnel.name] ?? [] : []))
const ends = computed(() => (props.tunnel ? text.ends(props.tunnel) : null))
const running = computed(() => props.tunnel?.state === 'running')
// Nothing to chart for a tunnel that is down and never moved a byte.
const hasTraffic = computed(() => running.value || (counters.value ? counters.value.down_bytes + counters.value.up_bytes > 0 : false))

const toneColor = computed(() => {
  const tun = props.tunnel
  if (!tun || tun.state === 'error') return 'var(--color-state-error-border)'
  if (tun.state !== 'running') return 'var(--color-state-stopped-dot)'
  return tun.direction === '-L' ? 'var(--color-dir-local)' : 'var(--color-dir-remote)'
})

const usage = computed(() => (props.tunnel ? traffic.usage?.tunnels[props.tunnel.name] : undefined))

function periodStat(label: string, u?: UsageTotals) {
  const down = u?.down_bytes ?? 0
  const up = u?.up_bytes ?? 0
  return { label, value: formatBytes(down + up), sub: `↓ ${formatBytes(down)} · ↑ ${formatBytes(up)}` }
}

// Live rates first, then persisted totals by period (they survive restarts).
const stats = computed(() => {
  const c = counters.value
  const u = usage.value
  return [
    { label: t('detail.downNow'), value: formatRate(c?.down_rate ?? 0), sub: '' },
    { label: t('detail.upNow'), value: formatRate(c?.up_rate ?? 0), sub: '' },
    { label: t('detail.openConns'), value: String(c?.active_conns ?? 0), sub: t('detail.totalConnsSub', { n: c?.total_conns ?? 0 }) },
    periodStat(t('detail.today'), u?.today),
    periodStat(t('detail.thisMonth'), u?.month),
    periodStat(t('detail.allTime'), u?.all ?? (c ? { down_bytes: c.down_bytes, up_bytes: c.up_bytes } : undefined)),
  ]
})

// Chart range: the live five minutes from the stream, or persisted history.
type ChartRange = 'live' | UsageRange
const range = ref<ChartRange>('live')
const ranges = computed<{ key: ChartRange; label: string }[]>(() => [
  { key: 'live', label: t('detail.rangeLive') },
  { key: '24h', label: t('detail.range24h') },
  { key: '30d', label: t('detail.range30d') },
  { key: '1y', label: t('detail.range1y') },
])
const usageHistory = ref<UsageHistory | null>(null)
const historyLoading = ref(false)

async function loadHistory() {
  if (!props.tunnel || !props.show) return
  // Keep the today/month/all tiles in step with the chart being shown.
  void traffic.refreshUsage()
  if (range.value === 'live') return
  historyLoading.value = true
  try {
    usageHistory.value = await api.trafficHistory(range.value, props.tunnel.name)
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  } finally {
    historyLoading.value = false
  }
}

watch([range, () => props.tunnel?.name, () => props.show], () => { void loadHistory() })

const periodTotal = computed(() => (usageHistory.value?.points ?? []).reduce(
  (acc, [, up, down]) => ({ up: acc.up + up, down: acc.down + down }), { up: 0, down: 0 },
))

const command = ref('')
const commandOpen = ref(false)
const commandLoading = ref(false)

watch(() => props.tunnel?.name, () => {
  command.value = ''
  commandOpen.value = false
  range.value = 'live'
  usageHistory.value = null
})

async function toggleCommand() {
  commandOpen.value = !commandOpen.value
  if (!commandOpen.value || command.value || !props.tunnel) return
  commandLoading.value = true
  try {
    command.value = (await api.getTunnelCommand(props.tunnel.name)).command
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
    commandOpen.value = false
  } finally {
    commandLoading.value = false
  }
}

async function copy(value: string, done: string) {
  const ok = await copyText(value)
  if (ok) message.success(done)
  else message.error(t('common.copyFailed'))
}

function act(action: TunnelCardAction) {
  if (props.tunnel) emit('action', action, props.tunnel)
}
</script>

<template>
  <n-drawer :show="show" :width="width" placement="right" @update:show="(v: boolean) => emit('update:show', v)">
    <n-drawer-content v-if="tunnel && ends" closable :body-content-style="{ padding: '18px 20px' }">
      <template #header>
        <div class="dh">
          <span class="dir" :class="tunnel.direction === '-L' ? 'local' : 'remote'">{{ text.kind(tunnel) }}</span>
          <span class="dname">{{ tunnel.name }}</span>
        </div>
      </template>

      <div class="body">
        <section class="status-row">
          <span class="status" :style="{ '--tone': toneColor }"><span class="dot" />{{ running ? t('overview.statusRunning') : tunnel.state === 'error' ? t('overview.statusError') : t('overview.statusStopped') }}</span>
          <n-switch :value="running" :loading="busy" @update:value="(on: boolean) => emit('toggle', tunnel!, on)" />
        </section>

        <section class="route">
          <div class="end">
            <span class="where">{{ ends.entryWhere }}</span>
            <code>{{ ends.entry }}</code>
            <n-button size="tiny" tertiary @click="copy(connectAddress(tunnel, remote), t('overview.addressCopied', { address: connectAddress(tunnel, remote) }))">{{ t('overview.copyAddress') }}</n-button>
          </div>
          <div class="arrow" :style="{ color: toneColor }" aria-hidden="true">
            <svg viewBox="0 0 20 40"><path d="M10 4v30m-6-6 6 6 6-6" /></svg>
          </div>
          <div class="end">
            <span class="where">{{ ends.serviceWhere }}</span>
            <code>{{ ends.service }}</code>
          </div>
        </section>

        <TunnelProblem v-if="tunnel.state === 'error'" :tunnel="tunnel" :remote="remote" @action="act" />

        <section class="block">
          <h4>{{ t('detail.traffic') }}</h4>
          <p v-if="tunnel.direct" class="muted">{{ t('traffic.direct') }}</p>
          <p v-else-if="!hasTraffic" class="muted">{{ t('detail.noTraffic') }}</p>
          <template v-else>
            <div class="ranges" role="tablist">
              <button v-for="r in ranges" :key="r.key" type="button" role="tab" :aria-selected="range === r.key" :class="{ active: range === r.key }" @click="range = r.key">{{ r.label }}</button>
            </div>
            <template v-if="range === 'live'">
              <TrafficChart :points="points" :color="toneColor" :now-label="t('detail.now')" :ago-label="t('detail.fiveMinAgo')" />
              <div class="legend"><span class="sw down" :style="{ background: toneColor }" />{{ t('traffic.down') }} <span class="sw up" :style="{ borderColor: toneColor }" />{{ t('traffic.up') }}</div>
            </template>
            <template v-else>
              <UsageChart :history="usageHistory" :color="toneColor" :locale="locale" :class="{ loading: historyLoading }" />
              <div class="legend">
                <span class="sw bar" :style="{ background: toneColor }" />{{ t('traffic.down') }}
                <span class="sw bar light" :style="{ background: toneColor }" />{{ t('traffic.up') }}
                <span class="period">{{ t('detail.periodTotal', { down: formatBytes(periodTotal.down), up: formatBytes(periodTotal.up) }) }}</span>
              </div>
            </template>
            <dl class="stats">
              <div v-for="s in stats" :key="s.label" :title="s.sub"><dt>{{ s.label }}</dt><dd>{{ s.value }}</dd><dd v-if="s.sub" class="sub">{{ s.sub }}</dd></div>
            </dl>
          </template>
        </section>

        <section class="block">
          <h4>{{ t('detail.settings') }}</h4>
          <dl class="facts">
            <div><dt>{{ t('detail.server') }}</dt><dd>{{ tunnel.remote }}<template v-if="remote"> · <code>{{ remote.user }}@{{ remote.host }}</code></template></dd></div>
            <div><dt>{{ t('wizard.keepAlive') }}</dt><dd>{{ tunnel.auto_start ? t('detail.yes') : t('detail.no') }}</dd></div>
            <div><dt>{{ t('tunnels.fields.meterTraffic') }}</dt><dd>{{ tunnel.direct ? t('detail.no') : t('detail.yes') }}</dd></div>
            <div v-if="tunnel.description"><dt>{{ t('tunnels.fields.description') }}</dt><dd>{{ tunnel.description }}</dd></div>
            <div v-if="tunnel.ssh_options?.length"><dt>SSH</dt><dd><code>{{ tunnel.ssh_options.join(' ') }}</code></dd></div>
          </dl>
        </section>

        <section class="block">
          <button type="button" class="disclose" :aria-expanded="commandOpen" @click="toggleCommand">
            <span class="chev" :class="{ open: commandOpen }">›</span>{{ t('detail.command') }}
          </button>
          <div v-if="commandOpen" class="command">
            <p class="muted">{{ t('tunnels.commandHelp') }}</p>
            <p v-if="commandLoading" class="muted">{{ t('common.loading') }}</p>
            <template v-else-if="command">
              <pre>{{ command }}</pre>
              <n-button size="tiny" secondary @click="copy(command, t('common.copied'))">{{ t('common.copy') }}</n-button>
            </template>
          </div>
        </section>
      </div>

      <template #footer>
        <div class="foot">
          <n-button quaternary type="error" @click="act('delete')">{{ t('common.delete') }}</n-button>
          <div class="right">
            <n-button v-if="tunnel.state !== 'stopped'" secondary @click="act('restart')">{{ t('common.restart') }}</n-button>
            <n-button type="primary" @click="act('edit')">{{ t('common.edit') }}</n-button>
          </div>
        </div>
      </template>
    </n-drawer-content>
  </n-drawer>
</template>

<style scoped>
.dh { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.dir { font-size: 11.5px; font-weight: 600; }
.dir.local { color: var(--color-dir-local-text); }
.dir.remote { color: var(--color-dir-remote-text); }
.dname { font-size: 17px; font-weight: 700; letter-spacing: -0.01em; color: var(--color-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.body { display: flex; flex-direction: column; gap: 18px; }
.status-row { display: flex; align-items: center; justify-content: space-between; }
.status { display: inline-flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; color: var(--color-text); }
.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--tone); }

.route { display: flex; flex-direction: column; align-items: stretch; padding: 12px 14px; border-radius: 12px; background: var(--color-surface-alt); border: 1px solid var(--color-border); }
.end { display: grid; grid-template-columns: 1fr auto; grid-template-rows: auto auto; column-gap: 8px; align-items: center; }
.end .where { grid-column: 1 / -1; font-size: 11.5px; color: var(--color-text-muted); }
.end code { font-family: var(--font-mono); font-size: 14px; font-weight: 600; color: var(--color-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.arrow { height: 34px; display: flex; justify-content: flex-start; padding-left: 6px; }
.arrow svg { width: 16px; height: 34px; fill: none; stroke: currentColor; stroke-width: 2; stroke-linecap: round; stroke-linejoin: round; }

.block { display: flex; flex-direction: column; gap: 10px; }
h4 { font-size: 12px; font-weight: 650; text-transform: uppercase; letter-spacing: 0.04em; color: var(--color-text-tertiary); }
.muted { font-size: 12px; color: var(--color-text-muted); line-height: 1.5; }
.legend { display: flex; align-items: center; gap: 6px; font-size: 11.5px; color: var(--color-text-tertiary); }
.legend .period { margin-left: auto; font-variant-numeric: tabular-nums; }
.sw.bar { width: 10px; height: 10px; border-radius: 2px; opacity: 0.85; }
.sw.bar.light { opacity: 0.35; }
.ranges { display: inline-flex; align-self: flex-start; border: 1px solid var(--color-border); border-radius: 8px; overflow: hidden; }
.ranges button { font: inherit; font-size: 12px; padding: 4px 11px; border: 0; cursor: pointer; background: var(--color-surface); color: var(--color-text-secondary); }
.ranges button + button { border-left: 1px solid var(--color-border); }
.ranges button.active { background: var(--color-text); color: var(--color-surface); }
.loading { opacity: 0.5; transition: opacity 0.15s; }
.sw { display: inline-block; width: 14px; height: 0; margin-left: 6px; }
.sw.down { height: 8px; border-radius: 2px; opacity: 0.6; }
.sw.up { border-top: 2px dashed; }
.stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.stats div { padding: 8px 10px; border-radius: 10px; background: var(--color-surface-alt); }
.stats dt { font-size: 11px; color: var(--color-text-muted); }
.stats dd.sub { font-size: 10.5px; font-weight: 400; color: var(--color-text-muted); margin-top: 1px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.stats dd { font-size: 14px; font-weight: 650; color: var(--color-text); font-variant-numeric: tabular-nums; margin-top: 2px; }
.facts { display: flex; flex-direction: column; gap: 8px; }
.facts div { display: grid; grid-template-columns: 96px 1fr; gap: 8px; font-size: 12.5px; }
.facts dt { color: var(--color-text-muted); }
.facts dd { color: var(--color-text-secondary); min-width: 0; overflow-wrap: anywhere; }
.facts code { font-family: var(--font-mono); font-size: 12px; }
.disclose { font: inherit; font-size: 12.5px; font-weight: 600; color: var(--color-text-secondary); background: none; border: 0; cursor: pointer; display: flex; align-items: center; gap: 6px; padding: 0; }
.chev { display: inline-block; transition: transform 0.15s; font-size: 16px; line-height: 1; }
.chev.open { transform: rotate(90deg); }
.command { display: flex; flex-direction: column; gap: 8px; align-items: flex-start; }
.command pre { width: 100%; font-family: var(--font-mono); font-size: 11.5px; line-height: 1.5; white-space: pre-wrap; word-break: break-all; padding: 10px; border-radius: 8px; background: var(--color-surface-alt); border: 1px solid var(--color-border); color: var(--color-text); }
.foot { display: flex; justify-content: space-between; width: 100%; gap: 8px; }
.foot .right { display: flex; gap: 8px; }
</style>
