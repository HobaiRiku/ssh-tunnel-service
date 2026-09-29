<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { NButton, NInput } from 'naive-ui'
import { RouterLink } from 'vue-router'
import type { Remote, TunnelStatus } from '@/api/client'
import FlowMap from '@/components/FlowMap.vue'
import TunnelCard from '@/components/TunnelCard.vue'
import type { TunnelCardAction } from '@/tunnelText'
import { useI18n } from '@/i18n'
import { useTrafficStore } from '@/stores/traffic'

// The home view: every tunnel as a card, grouped by the server it goes
// through, with a one-line health summary and quick filters on top.
const props = defineProps<{
  tunnels: TunnelStatus[]
  remotes: Remote[]
  loading: boolean
  busy: Set<string>
}>()

const emit = defineEmits<{
  (e: 'toggle', tunnel: TunnelStatus, on: boolean): void
  (e: 'action', action: TunnelCardAction, tunnel: TunnelStatus): void
  (e: 'add'): void
}>()

const { t } = useI18n()
const traffic = useTrafficStore()

type Filter = 'all' | 'running' | 'problems' | 'stopped'
const filter = ref<Filter>('all')
const query = ref('')

const counts = computed(() => ({
  all: props.tunnels.length,
  running: props.tunnels.filter((x) => x.state === 'running').length,
  problems: props.tunnels.filter((x) => x.state === 'error').length,
  stopped: props.tunnels.filter((x) => x.state === 'stopped').length,
}))

const filters = computed<{ key: Filter; label: string }[]>(() => [
  { key: 'all', label: t('overview.filterAll', { n: counts.value.all }) },
  { key: 'running', label: t('overview.filterRunning', { n: counts.value.running }) },
  { key: 'problems', label: t('overview.filterProblems', { n: counts.value.problems }) },
  { key: 'stopped', label: t('overview.filterStopped', { n: counts.value.stopped }) },
])

function matches(tun: TunnelStatus): boolean {
  if (filter.value === 'running' && tun.state !== 'running') return false
  if (filter.value === 'problems' && tun.state !== 'error') return false
  if (filter.value === 'stopped' && tun.state !== 'stopped') return false
  const q = query.value.trim().toLowerCase()
  if (!q) return true
  return [tun.name, tun.remote, tun.description, `${tun.bind_port}`, `${tun.target_host}:${tun.target_port}`]
    .some((v) => (v ?? '').toLowerCase().includes(q))
}

// Bring a tunnel's card into view and flash it — used by the flow map and
// after the wizard creates a tunnel.
async function focusTunnel(name: string) {
  filter.value = 'all'
  query.value = ''
  await nextTick()
  const el = document.querySelector<HTMLElement>(`[data-tunnel="${CSS.escape(name)}"]`)
  if (!el) return
  el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  el.classList.remove('flash')
  void el.offsetWidth
  el.classList.add('flash')
}

defineExpose({ focusTunnel })

// Problems float to the top of each group so they are never scrolled past.
const rank: Record<TunnelStatus['state'], number> = { error: 0, running: 1, stopped: 2 }

const groups = computed(() => {
  const byRemote = new Map<string, TunnelStatus[]>()
  for (const tun of props.tunnels) {
    if (!matches(tun)) continue
    const list = byRemote.get(tun.remote) ?? []
    list.push(tun)
    byRemote.set(tun.remote, list)
  }
  const remoteByName = new Map(props.remotes.map((r) => [r.name, r]))
  return [...byRemote.entries()]
    .map(([name, list]) => ({
      name,
      remote: remoteByName.get(name),
      tunnels: [...list].sort((a, b) => rank[a.state] - rank[b.state] || a.name.localeCompare(b.name)),
    }))
    .sort((a, b) => a.name.localeCompare(b.name))
})
</script>

<template>
  <div class="overview">
    <div v-if="!loading && remotes.length === 0" class="empty">
      <div class="empty-art" aria-hidden="true">
        <svg viewBox="0 0 120 60"><rect x="4" y="18" width="30" height="24" rx="6" /><rect x="86" y="18" width="30" height="24" rx="6" /><path d="M36 30h48" stroke-dasharray="4 4" /></svg>
      </div>
      <h3>{{ t('overview.noRemotesTitle') }}</h3>
      <p>{{ t('overview.noRemotesSub') }}</p>
      <RouterLink to="/remotes"><n-button type="primary">{{ t('overview.addRemote') }}</n-button></RouterLink>
    </div>

    <div v-else-if="!loading && tunnels.length === 0" class="empty">
      <div class="empty-art" aria-hidden="true">
        <svg viewBox="0 0 120 60"><rect x="4" y="18" width="30" height="24" rx="6" /><rect x="86" y="18" width="30" height="24" rx="6" /><path d="M36 30h48" /></svg>
      </div>
      <h3>{{ t('overview.noTunnelsTitle') }}</h3>
      <p>{{ t('overview.noTunnelsSub') }}</p>
      <n-button type="primary" @click="emit('add')">{{ t('overview.addTunnel') }}</n-button>
    </div>

    <template v-else>
      <FlowMap class="map" :tunnels="tunnels" :remotes="remotes" @select="focusTunnel" />
      <div class="toolbar">
        <div class="chips" role="tablist">
          <button
            v-for="f in filters"
            :key="f.key"
            type="button"
            role="tab"
            class="chip"
            :class="[`chip-${f.key}`, { active: filter === f.key, attention: f.key === 'problems' && counts.problems > 0 }]"
            :aria-selected="filter === f.key"
            @click="filter = f.key"
          >{{ f.label }}</button>
        </div>
        <n-input v-model:value="query" class="search" size="small" clearable round :placeholder="t('overview.search')" />
      </div>

      <section v-for="g in groups" :key="g.name" class="group">
        <header class="group-head">
          <span class="server-icon" aria-hidden="true">
            <svg viewBox="0 0 20 20"><rect x="3" y="3.5" width="14" height="5.5" rx="1.5" /><rect x="3" y="11" width="14" height="5.5" rx="1.5" /><circle cx="6" cy="6.25" r=".9" /><circle cx="6" cy="13.75" r=".9" /></svg>
          </span>
          <h2 class="group-name">{{ g.name }}</h2>
          <span v-if="g.remote" class="group-host">{{ g.remote.user }}@{{ g.remote.host }}{{ g.remote.port !== 22 ? `:${g.remote.port}` : '' }}</span>
        </header>
        <div class="grid">
          <TunnelCard
            v-for="tun in g.tunnels"
            :key="tun.name"
            :tunnel="tun"
            :remote="g.remote"
            :counters="traffic.tunnels[tun.name]"
            :points="traffic.history[tun.name] ?? []"
            :busy="busy.has(tun.name)"
            @toggle="(x, on) => emit('toggle', x, on)"
            @action="(a, x) => emit('action', a, x)"
          />
        </div>
      </section>

      <p v-if="groups.length === 0" class="no-match">{{ t('overview.noMatch') }}</p>
    </template>
  </div>
</template>

<style scoped>
.overview { display: flex; flex-direction: column; gap: 22px; }
.toolbar { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.chips { display: flex; gap: 6px; flex-wrap: wrap; }
.chip {
  font: inherit; font-size: 12.5px; font-weight: 500;
  padding: 5px 12px; border-radius: 999px; cursor: pointer;
  border: 1px solid var(--color-border); background: var(--color-surface); color: var(--color-text-secondary);
  transition: background 0.15s, color 0.15s, border-color 0.15s;
  font-variant-numeric: tabular-nums;
}
.chip:hover { border-color: var(--color-border-strong); }
.chip.active { background: var(--color-text); color: var(--color-surface); border-color: var(--color-text); }
.chip-problems.attention:not(.active) { color: var(--color-danger); border-color: color-mix(in srgb, var(--color-danger) 45%, transparent); }
.search { width: 220px; }

.group { display: flex; flex-direction: column; gap: 10px; }
.group-head { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
.server-icon { align-self: center; display: inline-flex; color: var(--color-text-tertiary); }
.server-icon svg { width: 16px; height: 16px; fill: none; stroke: currentColor; stroke-width: 1.4; }
.server-icon circle { fill: currentColor; stroke: none; }
.group-name { font-size: 13px; font-weight: 650; color: var(--color-text); }
.group-host { font-family: var(--font-mono); font-size: 11.5px; color: var(--color-text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 12px; align-items: start; }

.empty { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 10px; padding: 72px 16px; }
.empty-art svg { width: 140px; height: 70px; fill: var(--color-surface-alt); stroke: var(--color-border-strong); stroke-width: 1.5; }
.empty h3 { font-size: 16px; font-weight: 650; color: var(--color-text); }
.empty p { font-size: 13px; color: var(--color-text-tertiary); max-width: 380px; line-height: 1.5; margin-bottom: 6px; }
.no-match { text-align: center; font-size: 13px; color: var(--color-text-muted); padding: 32px 0; }

@media (max-width: 640px) {
  .map { display: none; }
  .search { width: 100%; }
  .grid { grid-template-columns: 1fr; }
}
</style>
