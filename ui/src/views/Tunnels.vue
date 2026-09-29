<script setup lang="ts">
import { computed, h, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NButtonGroup,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, DataTableRowKey } from 'naive-ui'
import { api, getErrorMessage, type Tunnel, type TunnelStatus } from '@/api/client'
import TunnelOverview from '@/components/TunnelOverview.vue'
import TunnelWizard from '@/components/TunnelWizard.vue'
import TunnelDetail from '@/components/TunnelDetail.vue'
import { useRoute, useRouter } from 'vue-router'
import TunnelTopologyView from '@/components/TunnelTopologyView.vue'
import { topologyViewState } from '@/components/topologyViewState'
import { copyText } from '@/clipboard'
import { formatEndpoint, parseEndpoint, validateName } from '@/validation'
import { useI18n } from '@/i18n'
import { useRemotesStore } from '@/stores/remotes'
import { useTunnelsStore } from '@/stores/tunnels'
import { useTrafficStore } from '@/stores/traffic'
import { formatRate } from '@/format'
import { useTunnelText, type TunnelCardAction } from '@/tunnelText'

const tunnelStore = useTunnelsStore()
const remoteStore = useRemotesStore()
const message = useMessage()
const dialog = useDialog()
const { t } = useI18n()
const trafficStore = useTrafficStore()
const route = useRoute()
const router = useRouter()
const tunnelText = useTunnelText()

const showModal = ref(false)
const showWizard = ref(false)
const wizardRemote = ref('')
const overviewRef = ref<InstanceType<typeof TunnelOverview> | null>(null)
const showCommandModal = ref(false)
const showDetail = ref(false)
const detailName = ref<string | null>(null)
const commandLoading = ref(false)
const commandTunnelLabel = ref('')
const commandValue = ref('')
const editingName = ref<string | null>(null)
type ViewMode = 'overview' | 'topology' | 'table'
const viewModeKey = 'ssh-tunnel-service.tunnels.view'
const viewMode = ref<ViewMode>(loadViewMode())
watch(viewMode, (mode) => {
  try { localStorage.setItem(viewModeKey, mode) } catch { /* storage unavailable */ }
})

function loadViewMode(): ViewMode {
  try {
    const stored = localStorage.getItem(viewModeKey)
    if (stored === 'overview' || stored === 'topology' || stored === 'table') return stored
  } catch { /* storage unavailable */ }
  return 'overview'
}

// Tunnels with a start/stop in flight, so their card switch shows a spinner.
const busy = ref(new Set<string>())
const selectedTunnelName = ref<string | null>(null)
const checkedRowKeys = ref<DataTableRowKey[]>([])

// Combined "host:port" inputs (item: simplify listen/target entry).
const bindInput = ref('')
const targetInput = ref('')

const submitted = ref(false)
const errors = reactive({ name: '', remote: '', bind: '', target: '' })

function createDefaultTunnel(): Tunnel {
  return {
    name: '',
    remote: '',
    direction: '-L',
    bind_address: '127.0.0.1',
    bind_port: 0,
    target_host: '',
    target_port: 0,
    ssh_options: [],
    auto_start: false,
    direct: false,
    description: '',
  }
}

function toTunnelForm(tunnel: Tunnel | TunnelStatus): Tunnel {
  return {
    name: tunnel.name,
    remote: tunnel.remote,
    direction: tunnel.direction,
    bind_address: tunnel.bind_address,
    bind_port: tunnel.bind_port,
    target_host: tunnel.target_host,
    target_port: tunnel.target_port,
    ssh_options: Array.isArray(tunnel.ssh_options) ? [...tunnel.ssh_options] : [],
    auto_start: tunnel.auto_start,
    direct: tunnel.direct ?? false,
    description: tunnel.description,
  }
}

const form = ref<Tunnel>(createDefaultTunnel())
const remoteOptions = computed(() => remoteStore.remotes.map((remote) => ({ label: `${remote.name} (${remote.host})`, value: remote.name })))

type TunnelDirection = Tunnel['direction']
type DirectionMeta = { value: TunnelDirection; code: string; title: string; summary: string; bindLabel: string; targetLabel: string; bindMeaning: string; targetMeaning: string }

const directionMeta = computed<Record<TunnelDirection, DirectionMeta>>(() => ({
  '-L': {
    value: '-L',
    code: '-L',
    title: t('tunnels.direction.localTitle'),
    summary: t('tunnels.direction.localSummary'),
    bindLabel: t('tunnels.direction.localBindLabel'),
    targetLabel: t('tunnels.direction.localTargetLabel'),
    bindMeaning: t('tunnels.direction.localBindMeaning'),
    targetMeaning: t('tunnels.direction.localTargetMeaning'),
  },
  '-R': {
    value: '-R',
    code: '-R',
    title: t('tunnels.direction.remoteTitle'),
    summary: t('tunnels.direction.remoteSummary'),
    bindLabel: t('tunnels.direction.remoteBindLabel'),
    targetLabel: t('tunnels.direction.remoteTargetLabel'),
    bindMeaning: t('tunnels.direction.remoteBindMeaning'),
    targetMeaning: t('tunnels.direction.remoteTargetMeaning'),
  },
}))

const directionList = computed<DirectionMeta[]>(() => [directionMeta.value['-L'], directionMeta.value['-R']])
const selectedDirection = computed(() => directionMeta.value[form.value.direction] ?? directionMeta.value['-L'])
const stateType: { [state in TunnelStatus['state']]: 'success' | 'default' | 'error' } = { running: 'success', stopped: 'default', error: 'error' }

function syncEndpointInputs() {
  bindInput.value = formatEndpoint(form.value.bind_address, form.value.bind_port)
  targetInput.value = formatEndpoint(form.value.target_host, form.value.target_port)
}

function runValidation(): boolean {
  errors.name = validateName(form.value.name, t) ?? ''
  errors.remote = form.value.remote ? '' : t('validation.remoteRequired')
  const bind = parseEndpoint(bindInput.value)
  errors.bind = bind ? '' : t('validation.bindInvalid')
  const target = parseEndpoint(targetInput.value)
  errors.target = target ? '' : t('validation.targetInvalid')
  if (bind) {
    form.value.bind_address = bind.host
    form.value.bind_port = bind.port
  }
  if (target) {
    form.value.target_host = target.host
    form.value.target_port = target.port
  }
  return !errors.name && !errors.remote && !errors.bind && !errors.target
}

// Metering is on unless the tunnel is direct; the form speaks in terms of the
// benefit ("measure traffic") rather than the mechanism.
const meterTraffic = computed({
  get: () => !form.value.direct,
  set: (on: boolean) => { form.value.direct = !on },
})

// Live feedback once the user has attempted a submit.
watch([form, bindInput, targetInput], () => {
  if (submitted.value) runValidation()
}, { deep: true })

function feedback(field: keyof typeof errors): string {
  return submitted.value ? errors[field] : ''
}
function status(field: keyof typeof errors): 'error' | undefined {
  return submitted.value && errors[field] ? 'error' : undefined
}

function resetForm() {
  form.value = createDefaultTunnel()
  submitted.value = false
  errors.name = errors.remote = errors.bind = errors.target = ''
  syncEndpointInputs()
}

// New tunnels start in the guided wizard; "Advanced" falls back to the full form.
function openAdd() {
  wizardRemote.value = ''
  if (viewMode.value === 'topology') {
    const active = topologyViewState.remoteId
    if (active && remoteStore.remotes.some((r) => r.name === active)) wizardRemote.value = active
  }
  showWizard.value = true
}

function openAdvancedFromWizard() {
  showWizard.value = false
  void openAddForm()
}

async function onWizardCreated(name: string) {
  if (viewMode.value !== 'overview') return
  await nextTick()
  await overviewRef.value?.focusTunnel(name)
}

async function openAddForm() {
  showModal.value = false
  editingName.value = null
  resetForm()
  // On a specific topology remote tab, pre-fill that remote (item 11).
  if (viewMode.value === 'topology') {
    const active = topologyViewState.remoteId
    if (active && remoteStore.remotes.some((r) => r.name === active)) {
      form.value.remote = active
    }
  }
  // With a single server there is nothing to choose.
  if (!form.value.remote && remoteStore.remotes.length === 1) {
    form.value.remote = remoteStore.remotes[0].name
  }
  await nextTick()
  showModal.value = true
}

async function openEdit(row: TunnelStatus) {
  showModal.value = false
  editingName.value = row.name
  form.value = toTunnelForm(row)
  submitted.value = false
  errors.name = errors.remote = errors.bind = errors.target = ''
  syncEndpointInputs()
  await nextTick()
  showModal.value = true
}

async function refresh() {
  await Promise.all([tunnelStore.fetchTunnels(), remoteStore.fetchRemotes()])
}

async function submitForm() {
  submitted.value = true
  if (!runValidation()) {
    message.error(t('validation.fixErrors'))
    return
  }
  try {
    if (editingName.value) {
      await tunnelStore.updateTunnel(editingName.value, form.value)
      message.success(t('tunnels.updated'))
    } else {
      await tunnelStore.addTunnel(form.value)
      message.success(t('tunnels.added'))
    }
    showModal.value = false
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doDelete(name: string) {
  try {
    await tunnelStore.deleteTunnel(name)
    message.success(t('tunnels.deleted'))
    if (selectedTunnelName.value === name) selectedTunnelName.value = null
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doStart(name: string) {
  try {
    await tunnelStore.startTunnel(name)
    message.info(t('tunnels.startRequested'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doStop(name: string) {
  try {
    await tunnelStore.stopTunnel(name)
    message.success(t('tunnels.stopped'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doRestart(name: string) {
  try {
    await tunnelStore.restartTunnel(name)
    message.info(t('tunnels.restartRequested'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function withBusy(name: string, run: () => Promise<void>) {
  busy.value = new Set(busy.value).add(name)
  try {
    await run()
  } finally {
    const next = new Set(busy.value)
    next.delete(name)
    busy.value = next
  }
}

function onCardToggle(tunnel: TunnelStatus, on: boolean) {
  void withBusy(tunnel.name, () => (on ? doStart(tunnel.name) : doStop(tunnel.name)))
}

function confirmDelete(tunnel: TunnelStatus) {
  dialog.warning({
    title: t('tunnels.deleteConfirm'),
    content: tunnelText.sentence(tunnel),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: () => doDelete(tunnel.name),
  })
}

const cardActions: Record<TunnelCardAction, (tunnel: TunnelStatus) => void> = {
  restart: (tunnel) => { void withBusy(tunnel.name, () => doRestart(tunnel.name)) },
  edit: (tunnel) => { void openEdit(tunnel) },
  command: (tunnel) => { void openCommand(tunnel) },
  copyName: (tunnel) => { void copyName(tunnel.name) },
  delete: confirmDelete,
  details: openDetails,
}

function onCardAction(action: TunnelCardAction, tunnel: TunnelStatus) {
  cardActions[action](tunnel)
}


async function copyName(name: string) {
  const ok = await copyText(name)
  if (ok) message.success(t('common.copied'))
  else message.error(t('common.copyFailed'))
}

// ---- Batch operations (item: bulk start/stop/delete) -------------------------
const selectedNames = computed(() => checkedRowKeys.value.map((k) => String(k)))

async function batchRun(action: (name: string) => Promise<void>) {
  const names = [...selectedNames.value]
  let failures = 0
  for (const name of names) {
    try {
      await action(name)
    } catch {
      failures += 1
    }
  }
  await tunnelStore.fetchTunnels()
  if (failures > 0) message.warning(`${failures}/${names.length}`)
  else message.success(t('tunnels.batchDone'))
}

async function batchStart() {
  await batchRun((name) => api.startTunnel(name))
}
async function batchStop() {
  await batchRun((name) => api.stopTunnel(name))
}
async function batchDelete() {
  await batchRun((name) => api.deleteTunnel(name))
  checkedRowKeys.value = []
}

// Clicking a tunnel (card title, topology node) opens its detail drawer. The
// drawer reads the live store entry, so state and traffic update in place.
function openDetails(tunnel: TunnelStatus) {
  detailName.value = tunnel.name
  selectedTunnelName.value = tunnel.name
  showDetail.value = true
}

const detailTunnel = computed(() => tunnelStore.tunnels.find((x) => x.name === detailName.value) ?? null)
const detailRemote = computed(() => remoteStore.remotes.find((r) => r.name === detailTunnel.value?.remote))

// Edit and delete close the drawer first so the dialog is not stacked on it.
function onDetailAction(action: TunnelCardAction, tunnel: TunnelStatus) {
  if (action === 'edit' || action === 'delete') showDetail.value = false
  onCardAction(action, tunnel)
}

async function openCommand(row: TunnelStatus) {
  showCommandModal.value = true
  commandTunnelLabel.value = row.name
  commandValue.value = ''
  commandLoading.value = true
  try {
    const preview = await api.getTunnelCommand(row.name)
    commandValue.value = preview.command
  } catch (error: unknown) {
    showCommandModal.value = false
    message.error(getErrorMessage(error))
  } finally {
    commandLoading.value = false
  }
}

const columns = computed<DataTableColumns<TunnelStatus>>(() => [
  { type: 'selection' },
  {
    title: t('tunnels.columns.name'),
    key: 'name',
    ellipsis: { tooltip: true },
    render: (row) => h('a', { class: 'name-link', onClick: () => openDetails(row) }, row.name),
  },
  { title: t('tunnels.columns.remote'), key: 'remote', width: 140 },
  {
    title: t('tunnels.columns.direction'),
    key: 'direction',
    width: 80,
    render: (row) => h('span', { style: `font-family:monospace;font-size:11px;padding:2px 6px;border-radius:4px;background:${row.direction === '-L' ? 'var(--color-tag-blue-bg)' : 'var(--color-tag-pink-bg)'};color:${row.direction === '-L' ? 'var(--color-tag-blue-text)' : 'var(--color-tag-pink-text)'}` }, row.direction),
  },
  { title: t('tunnels.columns.bind'), key: 'bind', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.bind_address}:${row.bind_port}`) },
  { title: t('tunnels.columns.target'), key: 'target', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.target_host}:${row.target_port}`) },
  {
    title: t('tunnels.columns.traffic'),
    key: 'traffic',
    width: 170,
    render: (row) => {
      if (row.direct) return h('span', { style: 'font-size:12px;color:var(--color-text-muted)' }, t('traffic.directShort'))
      const c = trafficStore.tunnels[row.name] ?? row.traffic
      return h('span', { style: 'font-size:12px;font-variant-numeric:tabular-nums;white-space:nowrap' }, `↓ ${formatRate(c?.down_rate ?? 0)}  ↑ ${formatRate(c?.up_rate ?? 0)}`)
    },
  },
  { title: t('tunnels.columns.state'), key: 'state', width: 90, render: (row) => h(NTag, { type: stateType[row.state], size: 'small', round: true }, { default: () => t(`common.${row.state}`) }) },
  {
    title: t('tunnels.columns.actions'),
    key: 'actions',
    width: 320,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        row.state !== 'running'
          ? h(NButton, { size: 'tiny', type: 'success', onClick: () => { void doStart(row.name) } }, { default: () => t('common.start') })
          : h(NButton, { size: 'tiny', type: 'warning', onClick: () => { void doStop(row.name) } }, { default: () => t('common.stop') }),
        h(NButton, { size: 'tiny', secondary: true, onClick: () => { void openEdit(row) } }, { default: () => t('common.edit') }),
        h(NButton, { size: 'tiny', tertiary: true, onClick: () => { void copyName(row.name) } }, { default: () => t('common.copyName') }),
        h(NButton, { size: 'tiny', tertiary: true, onClick: () => { void openCommand(row) } }, { default: () => t('common.ssh') }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.name) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => t('common.delete') }),
          default: () => t('tunnels.deleteConfirm'),
        }),
      ],
    }),
  },
])

onMounted(async () => {
  tunnelStore.startAutoRefresh()
  await refresh()
  // Deep link from a server card's "New tunnel here": /tunnels?new=<server>.
  const target = route.query.new
  if (typeof target === 'string') {
    wizardRemote.value = remoteStore.remotes.some((r) => r.name === target) ? target : ''
    showWizard.value = true
    void router.replace({ query: {} })
  }
})

onUnmounted(() => {
  tunnelStore.stopAutoRefresh()
})
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('tunnels.title') }}</span>
      <n-space align="center">
        <n-button-group>
          <n-button :type="viewMode === 'overview' ? 'primary' : 'default'" @click="viewMode = 'overview'">{{ t('common.overview') }}</n-button>
          <n-button :type="viewMode === 'topology' ? 'primary' : 'default'" @click="viewMode = 'topology'">{{ t('common.topology') }}</n-button>
          <n-button :type="viewMode === 'table' ? 'primary' : 'default'" @click="viewMode = 'table'">{{ t('common.table') }}</n-button>
        </n-button-group>
        <n-button secondary :loading="tunnelStore.loading || remoteStore.loading" @click="refresh">{{ t('common.refresh') }}</n-button>
        <n-button type="primary" @click="openAdd">{{ t('common.add') }}</n-button>
      </n-space>
    </div>

    <div class="page-body">
      <n-alert v-if="tunnelStore.error" type="error" :title="tunnelStore.error" style="margin-bottom:16px" />
      <div v-if="viewMode === 'table' && selectedNames.length > 0" class="batch-bar">
        <span class="batch-count">{{ t('common.selectedCount', { n: selectedNames.length }) }}</span>
        <n-space :size="8">
          <n-button size="small" type="success" @click="batchStart">{{ t('common.start') }}</n-button>
          <n-button size="small" type="warning" @click="batchStop">{{ t('common.stop') }}</n-button>
          <n-popconfirm @positive-click="batchDelete">
            <template #trigger><n-button size="small" type="error" ghost>{{ t('common.delete') }}</n-button></template>
            {{ t('tunnels.batchDeleteConfirm', { n: selectedNames.length }) }}
          </n-popconfirm>
          <n-button size="small" tertiary @click="checkedRowKeys = []">{{ t('common.clearSelection') }}</n-button>
        </n-space>
      </div>
      <TunnelOverview
        v-if="viewMode === 'overview'"
        :tunnels="tunnelStore.tunnels"
        :remotes="remoteStore.remotes"
        :loading="tunnelStore.loading || remoteStore.loading"
        :busy="busy"
        @toggle="onCardToggle"
        @action="onCardAction"
        ref="overviewRef"
        @add="openAdd"
      />
      <n-card v-else :bordered="false" class="content-card">
        <TunnelTopologyView
          v-if="viewMode === 'topology'"
          :tunnels="tunnelStore.tunnels"
          :remotes="remoteStore.remotes"
          :loading="tunnelStore.loading || remoteStore.loading"
          :selected-tunnel-name="selectedTunnelName"
          @select="openDetails"
        />
        <n-data-table
          v-else
          v-model:checked-row-keys="checkedRowKeys"
          :columns="columns"
          :data="tunnelStore.tunnels"
          :loading="tunnelStore.loading"
          :bordered="false"
          size="small"
          :row-key="(row: TunnelStatus) => row.name"
        />
      </n-card>
    </div>

    <TunnelWizard
      v-model:show="showWizard"
      :remotes="remoteStore.remotes"
      :initial-remote="wizardRemote"
      @created="onWizardCreated"
      @advanced="openAdvancedFromWizard"
    />

    <n-modal v-model:show="showModal" :title="editingName ? t('tunnels.editTitle') : t('tunnels.addTitle')" preset="dialog" style="width:680px;max-width:calc(100vw - 32px)">
      <n-form label-placement="left" label-width="120" style="margin-top:8px">
        <n-form-item :label="t('tunnels.fields.name')" :validation-status="status('name')" :feedback="feedback('name')">
          <n-input v-model:value="form.name" placeholder="e.g. db-forward" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.remote')" :validation-status="status('remote')" :feedback="feedback('remote')">
          <n-select v-model:value="form.remote" :options="remoteOptions" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.direction')" class="direction-form-item">
          <div class="direction-cards" role="radiogroup">
            <label
              v-for="meta in directionList"
              :key="meta.value"
              class="direction-card"
              :class="[meta.value === '-L' ? 'local' : 'remote', { active: form.direction === meta.value }]"
            >
              <input
                type="radio"
                name="tunnel-direction"
                class="direction-card-input"
                :value="meta.value"
                :checked="form.direction === meta.value"
                @change="form.direction = meta.value"
              />
              <span class="direction-card-radio" aria-hidden="true"></span>
              <span class="direction-card-body">
                <span class="direction-card-head">
                  <span class="direction-card-code">{{ meta.code }}</span>
                  <span class="direction-card-title">{{ meta.title }}</span>
                </span>
                <span class="direction-card-summary">{{ meta.summary }}</span>
              </span>
            </label>
          </div>
        </n-form-item>
        <n-form-item label=" " :show-feedback="false" class="direction-help-item">
          <div class="direction-help">
            <div class="direction-help-row">
              <span class="direction-help-tag">{{ selectedDirection.bindLabel }}</span>
              <span>{{ selectedDirection.bindMeaning }}</span>
            </div>
            <div class="direction-help-row">
              <span class="direction-help-tag direction-help-tag--target">{{ selectedDirection.targetLabel }}</span>
              <span>{{ selectedDirection.targetMeaning }}</span>
            </div>
          </div>
        </n-form-item>
        <n-form-item :label="selectedDirection.bindLabel" :validation-status="status('bind')" :feedback="feedback('bind')">
          <n-input v-model:value="bindInput" :placeholder="t('tunnels.listenPlaceholder')" />
        </n-form-item>
        <n-form-item :label="selectedDirection.targetLabel" :validation-status="status('target')" :feedback="feedback('target')">
          <n-input v-model:value="targetInput" :placeholder="t('tunnels.targetPlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.autoStart')">
          <n-space vertical :size="2" style="width:100%">
            <n-switch v-model:value="form.auto_start" />
            <n-text depth="3" style="font-size:12px">{{ t('tunnels.autoStartHint') }}</n-text>
          </n-space>
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.meterTraffic')">
          <n-space vertical :size="2" style="width:100%">
            <n-switch v-model:value="meterTraffic" />
            <n-text depth="3" style="font-size:12px">{{ meterTraffic ? t('tunnels.meterHint') : t('tunnels.directHint') }}</n-text>
          </n-space>
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.description')">
          <n-input v-model:value="form.description" type="textarea" :rows="2" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" @click="submitForm">{{ editingName ? t('common.save') : t('common.add') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <TunnelDetail
      v-model:show="showDetail"
      :tunnel="detailTunnel"
      :remote="detailRemote"
      :busy="detailTunnel ? busy.has(detailTunnel.name) : false"
      @toggle="onCardToggle"
      @action="onDetailAction"
    />

    <n-modal v-model:show="showCommandModal" :title="t('tunnels.commandTitle', { name: commandTunnelLabel })" preset="dialog" style="width:720px;max-width:calc(100vw - 32px)">
      <n-space vertical :size="12" style="margin-top:8px">
        <n-text depth="3">{{ t('tunnels.commandHelp') }}</n-text>
        <n-text v-if="commandLoading" depth="3">{{ t('common.loading') }}</n-text>
        <n-input v-else :value="commandValue" type="textarea" :rows="6" readonly />
      </n-space>
      <template #action>
        <n-space justify="end">
          <n-button v-if="!commandLoading && commandValue" secondary @click="void copyName(commandValue)">{{ t('common.copy') }}</n-button>
          <n-button @click="showCommandModal = false">{{ t('common.close') }}</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: var(--color-surface); border-bottom: 1px solid var(--color-border); flex-shrink: 0; gap: 12px; flex-wrap: wrap; }
.page-title { font-size: 14px; font-weight: 600; color: var(--color-text); }
.page-body { flex: 1; overflow: auto; padding: 20px; }
:deep(.name-link) { color: var(--color-text); font-weight: 600; cursor: pointer; }
:deep(.name-link:hover) { color: var(--color-accent); text-decoration: underline; }
@media (max-width: 640px) {
  .page-toolbar { padding: 10px 16px; }
  .page-body { padding: 16px; }
}
.content-card { border-radius: 12px; }
.batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
  padding: 8px 14px;
  background: var(--color-accent-bg);
  border: 1px solid var(--color-accent-border);
  border-radius: 10px;
}
.batch-count { font-size: 13px; font-weight: 600; color: var(--color-accent-strong); }
.direction-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  width: 100%;
}

.direction-card {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border: 1.5px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
}

.direction-card:hover { border-color: var(--color-border-strong); }
.direction-card.active.local { border-color: var(--color-accent-strong); background: var(--color-accent-bg); box-shadow: 0 0 0 3px rgba(29, 78, 216, 0.15); }
.direction-card.active.remote { border-color: #a21caf; background: var(--color-tag-pink-bg); box-shadow: 0 0 0 3px rgba(162, 28, 175, 0.15); }

.direction-card-input { position: absolute; opacity: 0; pointer-events: none; }

.direction-card-radio {
  width: 14px;
  height: 14px;
  margin-top: 2px;
  border-radius: 50%;
  border: 1.5px solid var(--color-border-strong);
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--color-surface);
}

.direction-card.active.local .direction-card-radio { border-color: var(--color-accent-strong); }
.direction-card.active.remote .direction-card-radio { border-color: #a21caf; }

.direction-card.active .direction-card-radio::after {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.direction-card.active.local .direction-card-radio::after { background: var(--color-accent-strong); }
.direction-card.active.remote .direction-card-radio::after { background: #a21caf; }

.direction-card-body { display: flex; flex-direction: column; gap: 4px; line-height: 1.35; min-width: 0; }
.direction-card-head { display: flex; align-items: center; gap: 6px; }
.direction-card-code {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--color-bg);
  color: var(--color-text-secondary);
}
.direction-card.local .direction-card-code { background: var(--color-tag-blue-bg); color: var(--color-tag-blue-text); }
.direction-card.remote .direction-card-code { background: var(--color-tag-pink-bg); color: var(--color-tag-pink-text); }
.direction-card-title { font-size: 13px; font-weight: 600; color: var(--color-text); }
.direction-card-summary { font-size: 12px; color: var(--color-text-tertiary); }

.direction-help-item {
  margin-top: -4px;
  margin-bottom: 12px;
}

.direction-help {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 12px;
  background: var(--color-surface-alt);
  border: 1px solid var(--color-border);
  font-size: 12px;
  color: var(--color-text-secondary);
}
.direction-help-row { display: flex; gap: 8px; align-items: flex-start; }
.direction-help-tag { min-width: 48px; display: inline-flex; justify-content: center; padding: 2px 8px; border-radius: 999px; background: var(--color-tag-blue-bg); color: var(--color-tag-blue-text); font-size: 11px; font-weight: 600; }
.direction-help-tag--target { background: var(--color-tag-purple-bg); color: var(--color-tag-purple-text); }
</style>
