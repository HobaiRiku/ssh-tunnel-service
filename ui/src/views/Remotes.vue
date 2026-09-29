<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSpace,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import { useRoute, useRouter } from 'vue-router'
import { getErrorMessage, type Remote, type TunnelStatus } from '@/api/client'
import { copyText } from '@/clipboard'
import { formatRate, isIdleRate } from '@/format'
import { useI18n } from '@/i18n'
import { useInstanceStore } from '@/stores/instance'
import { useKeysStore } from '@/stores/keys'
import { useRemotesStore } from '@/stores/remotes'
import { useTrafficStore } from '@/stores/traffic'
import { useTunnelsStore } from '@/stores/tunnels'
import { validateName } from '@/validation'

// Servers are what tunnels go *through*. Each card answers "can I use this
// server, and what runs over it?": how it logs in, the health of its tunnels,
// and their combined live traffic.
const remoteStore = useRemotesStore()
const keyStore = useKeysStore()
const tunnelStore = useTunnelsStore()
const traffic = useTrafficStore()
const instance = useInstanceStore()
const message = useMessage()
const dialog = useDialog()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const showModal = ref(false)
const editingName = ref<string | null>(null)
const submitted = ref(false)
const errors = reactive({ name: '', host: '' })

function emptyRemote(): Remote {
  return { name: '', host: '', port: 22, user: '', key: '', description: '' }
}
const form = ref<Remote>(emptyRemote())

const defaultKey = computed(() => keyStore.keys.find((k) => k.system_default)?.name ?? '')

// An unbound server logs in differently depending on who runs the service, so
// say which one applies here instead of a vague "default".
const unboundLabel = computed(() => instance.isSystem
  ? t('servers.unboundSystem', { key: defaultKey.value || '—' })
  : t('servers.unboundUser'))

const keyOptions = computed(() => [
  { label: unboundLabel.value, value: '' },
  ...keyStore.keys.map((key) => ({ label: t('servers.withKey', { key: key.name }), value: key.name })),
])

interface ServerView {
  remote: Remote
  tunnels: TunnelStatus[]
  running: number
  failing: number
  down: number
  up: number
  tone: 'ok' | 'error' | 'idle'
}

const servers = computed<ServerView[]>(() => remoteStore.remotes.map((remote) => {
  const tunnels = tunnelStore.tunnels.filter((x) => x.remote === remote.name)
  const running = tunnels.filter((x) => x.state === 'running').length
  const failing = tunnels.filter((x) => x.state === 'error').length
  const down = tunnels.reduce((sum, x) => sum + (traffic.tunnels[x.name]?.down_rate ?? 0), 0)
  const up = tunnels.reduce((sum, x) => sum + (traffic.tunnels[x.name]?.up_rate ?? 0), 0)
  const tone = failing > 0 ? 'error' : running > 0 ? 'ok' : 'idle'
  return { remote, tunnels, running, failing, down, up, tone }
}).sort((a, b) => a.remote.name.localeCompare(b.remote.name)))

function loginText(remote: Remote): string {
  return remote.key ? t('servers.withKey', { key: remote.key }) : unboundLabel.value
}

function sshTarget(remote: Remote): string {
  return `${remote.user ? `${remote.user}@` : ''}${remote.host}${remote.port !== 22 ? `:${remote.port}` : ''}`
}

function runValidation(): boolean {
  errors.name = validateName(form.value.name, t) ?? ''
  errors.host = form.value.host ? '' : t('validation.hostRequired')
  return !errors.name && !errors.host
}

watch(form, () => {
  if (submitted.value) runValidation()
}, { deep: true })

function feedback(field: keyof typeof errors): string {
  return submitted.value ? errors[field] : ''
}
function status(field: keyof typeof errors): 'error' | undefined {
  return submitted.value && errors[field] ? 'error' : undefined
}

function openAdd() {
  editingName.value = null
  form.value = emptyRemote()
  submitted.value = false
  errors.name = errors.host = ''
  showModal.value = true
}

function openEdit(row: Remote) {
  editingName.value = row.name
  form.value = { ...row }
  submitted.value = false
  errors.name = errors.host = ''
  showModal.value = true
}

async function submitForm() {
  submitted.value = true
  if (!runValidation()) {
    message.error(t('validation.fixErrors'))
    return
  }
  try {
    if (editingName.value) {
      await remoteStore.updateRemote(editingName.value, form.value)
      message.success(t('remotes.updated'))
    } else {
      await remoteStore.addRemote(form.value)
      message.success(t('remotes.added'))
    }
    showModal.value = false
    await tunnelStore.fetchTunnels()
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

function confirmDelete(view: ServerView) {
  // The service refuses to delete a server tunnels still use; say so up front
  // rather than letting the request fail.
  if (view.tunnels.length > 0) {
    dialog.info({
      title: t('servers.inUseTitle', { name: view.remote.name }),
      content: t('servers.inUseBody', { n: view.tunnels.length }),
      positiveText: t('common.close'),
    })
    return
  }
  dialog.warning({
    title: t('remotes.deleteConfirm'),
    content: `${view.remote.name} · ${sshTarget(view.remote)}`,
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await remoteStore.deleteRemote(view.remote.name)
        message.success(t('remotes.deleted'))
      } catch (error: unknown) {
        message.error(getErrorMessage(error))
      }
    },
  })
}

function newTunnel(remote: Remote) {
  void router.push({ path: '/tunnels', query: { new: remote.name } })
}

async function copyTarget(remote: Remote) {
  const cmd = `ssh ${remote.port !== 22 ? `-p ${remote.port} ` : ''}${remote.user ? `${remote.user}@` : ''}${remote.host}`
  const ok = await copyText(cmd)
  if (ok) message.success(t('servers.sshCopied', { cmd }))
  else message.error(t('common.copyFailed'))
}

function menu(view: ServerView): DropdownOption[] {
  return [
    { key: 'copy', label: t('servers.copySsh') },
    { key: 'copyName', label: t('common.copyName') },
    { type: 'divider', key: 'd' },
    { key: 'delete', label: t('common.delete') },
  ].map((o) => (o.key === 'delete' && view.tunnels.length > 0 ? { ...o, label: t('servers.deleteBlocked') } : o))
}

async function onMenu(key: string, view: ServerView) {
  if (key === 'copy') return copyTarget(view.remote)
  if (key === 'copyName') {
    const ok = await copyText(view.remote.name)
    if (ok) message.success(t('common.copied'))
    return
  }
  if (key === 'delete') confirmDelete(view)
}

onMounted(async () => {
  tunnelStore.startAutoRefresh()
  void instance.load()
  await Promise.all([remoteStore.fetchRemotes(), keyStore.fetchKeys(), tunnelStore.fetchTunnels()])
  // Deep link from a failed tunnel's "Edit server" fix: /remotes?edit=<name>.
  const target = route.query.edit
  if (typeof target === 'string') {
    const remote = remoteStore.remotes.find((r) => r.name === target)
    if (remote) openEdit(remote)
    void router.replace({ query: {} })
  }
})

onUnmounted(() => tunnelStore.stopAutoRefresh())
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('remotes.title') }}</span>
      <n-button type="primary" @click="openAdd">{{ t('servers.add') }}</n-button>
    </div>

    <div class="page-body">
      <n-alert v-if="remoteStore.error" type="error" :title="remoteStore.error" style="margin-bottom:16px" />

      <div v-if="!remoteStore.loading && servers.length === 0" class="empty">
        <h3>{{ t('overview.noRemotesTitle') }}</h3>
        <p>{{ t('overview.noRemotesSub') }}</p>
        <n-button type="primary" @click="openAdd">{{ t('servers.add') }}</n-button>
      </div>

      <div v-else class="grid">
        <article v-for="view in servers" :key="view.remote.name" class="server" :class="`tone-${view.tone}`">
          <header class="head">
            <span class="icon" aria-hidden="true">
              <svg viewBox="0 0 20 20"><rect x="3" y="3.5" width="14" height="5.5" rx="1.5" /><rect x="3" y="11" width="14" height="5.5" rx="1.5" /><circle cx="6" cy="6.25" r=".9" /><circle cx="6" cy="13.75" r=".9" /></svg>
            </span>
            <div class="titles">
              <h3 class="name">{{ view.remote.name }}</h3>
              <code class="target">{{ sshTarget(view.remote) }}</code>
            </div>
            <n-dropdown trigger="click" :options="menu(view)" placement="bottom-end" @select="(k: string) => onMenu(k, view)">
              <n-button size="small" quaternary :aria-label="t('overview.more')">
                <svg class="more" viewBox="0 0 20 20" aria-hidden="true"><circle cx="4" cy="10" r="1.6" /><circle cx="10" cy="10" r="1.6" /><circle cx="16" cy="10" r="1.6" /></svg>
              </n-button>
            </n-dropdown>
          </header>

          <p v-if="view.remote.description" class="desc">{{ view.remote.description }}</p>

          <dl class="facts">
            <div><dt>{{ t('servers.login') }}</dt><dd>{{ loginText(view.remote) }}</dd></div>
            <div>
              <dt>{{ t('servers.tunnels') }}</dt>
              <dd class="health">
                <template v-if="view.tunnels.length === 0">{{ t('servers.noTunnels') }}</template>
                <template v-else>
                  <span>{{ t('servers.tunnelCount', { n: view.tunnels.length }) }}</span>
                  <span v-if="view.running" class="pill ok">{{ t('servers.running', { n: view.running }) }}</span>
                  <span v-if="view.failing" class="pill error">{{ t('servers.failing', { n: view.failing }) }}</span>
                </template>
              </dd>
            </div>
            <div v-if="view.running">
              <dt>{{ t('tunnels.columns.traffic') }}</dt>
              <dd class="rates">
                <span :class="{ idle: isIdleRate(view.down) }">↓ {{ formatRate(view.down) }}</span>
                <span :class="{ idle: isIdleRate(view.up) }">↑ {{ formatRate(view.up) }}</span>
              </dd>
            </div>
          </dl>

          <footer class="foot">
            <n-button size="small" secondary @click="openEdit(view.remote)">{{ t('common.edit') }}</n-button>
            <n-button size="small" type="primary" ghost @click="newTunnel(view.remote)">{{ t('servers.newTunnel') }}</n-button>
          </footer>
        </article>
      </div>
    </div>

    <n-modal v-model:show="showModal" :title="editingName ? t('remotes.editTitle') : t('remotes.addTitle')" preset="dialog" style="width:540px;max-width:calc(100vw - 32px)">
      <n-form label-placement="top" style="margin-top:8px">
        <n-form-item :label="t('remotes.fields.name')" :validation-status="status('name')" :feedback="feedback('name')">
          <n-input v-model:value="form.name" :placeholder="t('servers.namePlaceholder')" />
        </n-form-item>
        <div class="row">
          <n-form-item class="grow" :label="t('remotes.fields.host')" :validation-status="status('host')" :feedback="feedback('host')">
            <n-input v-model:value="form.host" placeholder="203.0.113.10 / bastion.example.com" />
          </n-form-item>
          <n-form-item class="port" :label="t('remotes.fields.port')">
            <n-input-number v-model:value="form.port" :min="1" :max="65535" :show-button="false" />
          </n-form-item>
        </div>
        <n-form-item :label="t('remotes.fields.user')">
          <n-input v-model:value="form.user" placeholder="ubuntu" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.key')">
          <n-space vertical :size="4" style="width:100%">
            <n-select v-model:value="form.key" :options="keyOptions" />
            <n-text depth="3" style="font-size:12px">{{ form.key ? t('servers.keyHintBound') : instance.isSystem ? t('servers.keyHintSystem') : t('servers.keyHintUser') }}</n-text>
          </n-space>
        </n-form-item>
        <n-form-item :label="t('remotes.fields.description')">
          <n-input v-model:value="form.description" :placeholder="t('servers.descriptionPlaceholder')" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" @click="submitForm">{{ editingName ? t('common.save') : t('common.add') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: var(--color-surface); border-bottom: 1px solid var(--color-border); flex-shrink: 0; }
.page-title { font-size: 14px; font-weight: 600; color: var(--color-text); }
.page-body { flex: 1; overflow: auto; padding: 20px; }

.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 12px; align-items: start; }
.server {
  --tone: var(--color-state-stopped-dot);
  display: flex; flex-direction: column; gap: 12px;
  padding: 14px 16px 12px; border-radius: 14px;
  background: var(--color-surface); border: 1px solid var(--color-border);
  box-shadow: 0 1px 2px var(--color-shadow-soft);
  transition: box-shadow 0.2s, border-color 0.2s;
}
.server:hover { box-shadow: 0 6px 20px var(--color-shadow-soft); border-color: var(--color-border-strong); }
.tone-ok { --tone: var(--color-dir-local); }
.tone-error { --tone: var(--color-state-error-border); }
.head { display: flex; align-items: center; gap: 10px; min-width: 0; }
.icon {
  width: 34px; height: 34px; border-radius: 10px; flex-shrink: 0;
  display: inline-flex; align-items: center; justify-content: center;
  color: var(--tone); background: color-mix(in srgb, var(--tone) 14%, transparent);
}
.icon svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.5; }
.icon circle { fill: currentColor; stroke: none; }
.titles { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.name { font-size: 14.5px; font-weight: 650; color: var(--color-text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.target { font-family: var(--font-mono); font-size: 12px; color: var(--color-text-tertiary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.more { width: 16px; height: 16px; fill: currentColor; }
.desc { font-size: 12.5px; color: var(--color-text-secondary); line-height: 1.5; }

.facts { display: flex; flex-direction: column; gap: 8px; }
.facts > div { display: grid; grid-template-columns: 64px 1fr; gap: 8px; align-items: baseline; }
.facts dt { font-size: 11.5px; color: var(--color-text-muted); }
.facts dd { font-size: 12.5px; color: var(--color-text-secondary); min-width: 0; }
.health { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
.pill { font-size: 11px; font-weight: 600; padding: 1px 8px; border-radius: 999px; }
.pill.ok { color: var(--color-dir-local-text); background: var(--color-dir-local-soft); }
.pill.error { color: var(--color-danger); background: var(--color-state-error-bg); }
.rates { display: flex; gap: 12px; font-variant-numeric: tabular-nums; }
.rates .idle { color: var(--color-text-muted); }

.foot { display: flex; justify-content: flex-end; gap: 8px; border-top: 1px solid var(--color-divider-soft); padding-top: 10px; }

.row { display: flex; gap: 12px; }
.row .grow { flex: 1; }
.row .port { width: 110px; }

.empty { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 10px; padding: 72px 16px; }
.empty h3 { font-size: 16px; font-weight: 650; color: var(--color-text); }
.empty p { font-size: 13px; color: var(--color-text-tertiary); max-width: 380px; line-height: 1.5; margin-bottom: 6px; }

@media (max-width: 640px) {
  .page-toolbar { padding: 10px 16px; }
  .page-body { padding: 16px; }
  .grid { grid-template-columns: 1fr; }
}
</style>
