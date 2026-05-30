<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NButtonGroup,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSwitch,
  NTag,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns, SelectOption } from 'naive-ui'
import { api, getErrorMessage, type Tunnel, type TunnelStatus } from '@/api/client'
import TunnelTopologyView from '@/components/TunnelTopologyView.vue'
import { useI18n } from '@/i18n'
import { useRemotesStore } from '@/stores/remotes'
import { useTunnelsStore } from '@/stores/tunnels'

const tunnelStore = useTunnelsStore()
const remoteStore = useRemotesStore()
const message = useMessage()
const { t } = useI18n()

const showModal = ref(false)
const showCommandModal = ref(false)
const showActionModal = ref(false)
const commandLoading = ref(false)
const commandTunnelLabel = ref('')
const commandValue = ref('')
const editingId = ref<string | null>(null)
const viewMode = ref<'topology' | 'table'>('topology')
const activeTunnel = ref<TunnelStatus | null>(null)
const selectedTunnelId = ref<string | null>(null)

function createDefaultTunnel(): Tunnel {
  return {
    id: '',
    name: '',
    remote_id: '',
    direction: '-L',
    bind_address: '127.0.0.1',
    bind_port: 0,
    target_host: '',
    target_port: 0,
    ssh_options: [],
    auto_start: false,
    description: '',
  }
}

function toTunnelForm(tunnel: Tunnel | TunnelStatus): Tunnel {
  return {
    id: tunnel.id,
    name: tunnel.name,
    remote_id: tunnel.remote_id,
    direction: tunnel.direction,
    bind_address: tunnel.bind_address,
    bind_port: tunnel.bind_port,
    target_host: tunnel.target_host,
    target_port: tunnel.target_port,
    ssh_options: [...tunnel.ssh_options],
    auto_start: tunnel.auto_start,
    description: tunnel.description,
  }
}

const form = ref<Tunnel>(createDefaultTunnel())
const remoteOptions = computed(() => remoteStore.remotes.map((remote) => ({ label: `${remote.name} (${remote.host})`, value: remote.id })))
const remoteNameMap = computed(() => new Map(remoteStore.remotes.map((remote) => [remote.id, remote.name] as const)))

type TunnelDirection = Tunnel['direction']
type DirectionMeta = { value: TunnelDirection; code: string; title: string; summary: string; bindMeaning: string; targetMeaning: string }
type DirectionOption = SelectOption & { value: TunnelDirection; label: string; meta: DirectionMeta }

const directionMeta = computed<Record<TunnelDirection, DirectionMeta>>(() => ({
  '-L': {
    value: '-L',
    code: '-L',
    title: t('tunnels.direction.localTitle'),
    summary: t('tunnels.direction.localSummary'),
    bindMeaning: t('tunnels.direction.localBindMeaning'),
    targetMeaning: t('tunnels.direction.localTargetMeaning'),
  },
  '-R': {
    value: '-R',
    code: '-R',
    title: t('tunnels.direction.remoteTitle'),
    summary: t('tunnels.direction.remoteSummary'),
    bindMeaning: t('tunnels.direction.remoteBindMeaning'),
    targetMeaning: t('tunnels.direction.remoteTargetMeaning'),
  },
}))

const dirOptions = computed<DirectionOption[]>(() => Object.values(directionMeta.value).map((meta) => ({
  value: meta.value,
  label: `${meta.code} ${meta.title} — ${meta.summary}`,
  meta,
})))

function isDirectionOption(option: SelectOption): option is DirectionOption {
  return typeof option.value === 'string' && typeof option.label === 'string' && typeof option.meta === 'object' && option.meta !== null
}

function renderDirectionLabel(option: SelectOption) {
  if (!isDirectionOption(option)) return String(option.label ?? '')
  return h('div', { style: 'display:flex;flex-direction:column;gap:2px;line-height:1.4;padding:2px 0' }, [
    h('div', { style: 'font-family:monospace;font-weight:600;font-size:13px;color:#1e293b' }, `${option.meta.code} ${option.meta.title}`),
    h('div', { style: 'font-size:12px;color:#64748b;white-space:normal' }, option.meta.summary),
  ])
}

const selectedDirection = computed(() => directionMeta.value[form.value.direction])
const stateType: { [state in TunnelStatus['state']]: 'success' | 'default' | 'error' } = { running: 'success', stopped: 'default', error: 'error' }

function resetForm() {
  form.value = createDefaultTunnel()
}

function openAdd() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEdit(row: TunnelStatus) {
  editingId.value = row.id
  form.value = toTunnelForm(row)
  showModal.value = true
}

async function refresh() {
  await Promise.all([tunnelStore.fetchTunnels(), remoteStore.fetchRemotes()])
}

async function submitForm() {
  try {
    if (editingId.value) {
      await tunnelStore.updateTunnel(editingId.value, form.value)
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

async function doDelete(id: string) {
  try {
    await tunnelStore.deleteTunnel(id)
    message.success(t('tunnels.deleted'))
    if (selectedTunnelId.value === id) selectedTunnelId.value = null
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doStart(id: string) {
  try {
    await tunnelStore.startTunnel(id)
    message.info(t('tunnels.startRequested'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doStop(id: string) {
  try {
    await tunnelStore.stopTunnel(id)
    message.success(t('tunnels.stopped'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

function openActions(tunnel: TunnelStatus) {
  activeTunnel.value = tunnel
  selectedTunnelId.value = tunnel.id
  showActionModal.value = true
}

function closeActions() {
  showActionModal.value = false
}

const actionState = computed(() => activeTunnel.value?.state ?? 'stopped')

async function actionStart() {
  if (!activeTunnel.value) return
  closeActions()
  await doStart(activeTunnel.value.id)
}

async function actionStop() {
  if (!activeTunnel.value) return
  closeActions()
  await doStop(activeTunnel.value.id)
}

function actionEdit() {
  if (!activeTunnel.value) return
  const tunnel = activeTunnel.value
  closeActions()
  openEdit(tunnel)
}

async function actionCommand() {
  if (!activeTunnel.value) return
  const tunnel = activeTunnel.value
  closeActions()
  await openCommand(tunnel)
}

async function actionDelete() {
  if (!activeTunnel.value) return
  const id = activeTunnel.value.id
  closeActions()
  await doDelete(id)
}

async function openCommand(row: TunnelStatus) {
  showCommandModal.value = true
  commandTunnelLabel.value = row.name || row.id
  commandValue.value = ''
  commandLoading.value = true
  try {
    const preview = await api.getTunnelCommand(row.id)
    commandValue.value = preview.command
  } catch (error: unknown) {
    showCommandModal.value = false
    message.error(getErrorMessage(error))
  } finally {
    commandLoading.value = false
  }
}

const columns = computed<DataTableColumns<TunnelStatus>>(() => [
  { title: t('tunnels.columns.name'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('tunnels.columns.remote'), key: 'remote_id', width: 140, render: (row) => remoteNameMap.value.get(row.remote_id) ?? row.remote_id },
  {
    title: t('tunnels.columns.direction'),
    key: 'direction',
    width: 80,
    render: (row) => h('span', { style: `font-family:monospace;font-size:11px;padding:2px 6px;border-radius:4px;background:${row.direction === '-L' ? '#dbeafe' : '#fce7f3'};color:${row.direction === '-L' ? '#1d4ed8' : '#9d174d'}` }, row.direction),
  },
  { title: t('tunnels.columns.bind'), key: 'bind', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.bind_address}:${row.bind_port}`) },
  { title: t('tunnels.columns.target'), key: 'target', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.target_host}:${row.target_port}`) },
  { title: t('tunnels.columns.state'), key: 'state', width: 90, render: (row) => h(NTag, { type: stateType[row.state], size: 'small', round: true }, { default: () => t(`common.${row.state}`) }) },
  {
    title: t('tunnels.columns.actions'),
    key: 'actions',
    width: 270,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        row.state !== 'running'
          ? h(NButton, { size: 'tiny', type: 'success', onClick: () => { void doStart(row.id) } }, { default: () => t('common.start') })
          : h(NButton, { size: 'tiny', type: 'warning', onClick: () => { void doStop(row.id) } }, { default: () => t('common.stop') }),
        h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
        h(NButton, { size: 'tiny', tertiary: true, onClick: () => { void openCommand(row) } }, { default: () => t('common.ssh') }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => t('common.delete') }),
          default: () => t('tunnels.deleteConfirm'),
        }),
      ],
    }),
  },
])

onMounted(() => {
  void refresh()
})
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('tunnels.title') }}</span>
      <n-space align="center">
        <n-button-group>
          <n-button :type="viewMode === 'topology' ? 'primary' : 'default'" @click="viewMode = 'topology'">{{ t('common.topology') }}</n-button>
          <n-button :type="viewMode === 'table' ? 'primary' : 'default'" @click="viewMode = 'table'">{{ t('common.table') }}</n-button>
        </n-button-group>
        <n-button secondary :loading="tunnelStore.loading || remoteStore.loading" @click="refresh">{{ t('common.refresh') }}</n-button>
        <n-button type="primary" @click="openAdd">{{ t('common.add') }}</n-button>
      </n-space>
    </div>

    <div class="page-body">
      <n-alert v-if="tunnelStore.error" type="error" :title="tunnelStore.error" style="margin-bottom:16px" />
      <n-card :bordered="false" class="content-card">
        <TunnelTopologyView
          v-if="viewMode === 'topology'"
          :tunnels="tunnelStore.tunnels"
          :remotes="remoteStore.remotes"
          :loading="tunnelStore.loading || remoteStore.loading"
          :selected-tunnel-id="selectedTunnelId"
          @select="openActions"
        />
        <n-data-table
          v-else
          :columns="columns"
          :data="tunnelStore.tunnels"
          :loading="tunnelStore.loading"
          :bordered="false"
          size="small"
          :row-key="(row: TunnelStatus) => row.id"
        />
      </n-card>
    </div>

    <n-modal v-model:show="showModal" :title="editingId ? t('tunnels.editTitle') : t('tunnels.addTitle')" preset="dialog" style="width:560px">
      <n-form label-placement="left" label-width="120" style="margin-top:8px">
        <n-form-item v-if="!editingId" :label="t('tunnels.fields.id')">
          <n-input v-model:value="form.id" placeholder="e.g. db-forward" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.name')">
          <n-input v-model:value="form.name" :placeholder="t('tunnels.fields.name')" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.remote')">
          <n-select v-model:value="form.remote_id" :options="remoteOptions" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.direction')">
          <n-select v-model:value="form.direction" :options="dirOptions" :render-label="renderDirectionLabel" />
        </n-form-item>
        <n-form-item label=" " :show-feedback="false">
          <div class="direction-help">
            <div class="direction-help-row">
              <span class="direction-help-tag">Bind</span>
              <span>{{ selectedDirection.bindMeaning }}</span>
            </div>
            <div class="direction-help-row">
              <span class="direction-help-tag direction-help-tag--target">Target</span>
              <span>{{ selectedDirection.targetMeaning }}</span>
            </div>
          </div>
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.bindAddress')">
          <n-input v-model:value="form.bind_address" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.bindPort')">
          <n-input-number v-model:value="form.bind_port" :min="1" :max="65535" style="width:100%" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.targetHost')">
          <n-input v-model:value="form.target_host" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.targetPort')">
          <n-input-number v-model:value="form.target_port" :min="1" :max="65535" style="width:100%" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.autoStart')">
          <n-switch v-model:value="form.auto_start" />
        </n-form-item>
        <n-form-item :label="t('tunnels.fields.description')">
          <n-input v-model:value="form.description" type="textarea" :rows="2" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showModal = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" @click="submitForm">{{ editingId ? t('common.save') : t('common.add') }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showActionModal" :title="activeTunnel ? t('tunnels.actionTitle', { name: activeTunnel.name || activeTunnel.id }) : t('tunnels.title')" preset="dialog" style="width:460px">
      <div v-if="activeTunnel" class="action-summary">
        <div class="action-summary-row"><span class="action-summary-label">{{ t('tunnels.fields.remote') }}</span><span class="action-summary-value">{{ remoteNameMap.get(activeTunnel.remote_id) ?? activeTunnel.remote_id }}</span></div>
        <div class="action-summary-row"><span class="action-summary-label">{{ t('tunnels.fields.direction') }}</span><span class="action-summary-value"><span class="dir-chip" :class="activeTunnel.direction === '-L' ? 'local' : 'remote'">{{ activeTunnel.direction }}</span></span></div>
        <div class="action-summary-row"><span class="action-summary-label">{{ t('tunnels.columns.bind') }}</span><span class="action-summary-value mono">{{ activeTunnel.bind_address }}:{{ activeTunnel.bind_port }}</span></div>
        <div class="action-summary-row"><span class="action-summary-label">{{ t('tunnels.columns.target') }}</span><span class="action-summary-value mono">{{ activeTunnel.target_host }}:{{ activeTunnel.target_port }}</span></div>
        <div class="action-summary-row">
          <span class="action-summary-label">{{ t('tunnels.fields.state') }}</span>
          <span class="action-summary-value">
            <n-tag :type="stateType[activeTunnel.state]" size="small" round>{{ t(`common.${activeTunnel.state}`) }}</n-tag>
            <span v-if="activeTunnel.error" class="action-summary-error">{{ activeTunnel.error }}</span>
          </span>
        </div>
      </div>
      <template #action>
        <n-space justify="space-between" style="width:100%">
          <n-popconfirm @positive-click="actionDelete">
            <template #trigger><n-button type="error" ghost>{{ t('common.delete') }}</n-button></template>
            {{ t('tunnels.deleteConfirm') }}
          </n-popconfirm>
          <n-space>
            <n-button @click="closeActions">{{ t('common.close') }}</n-button>
            <n-button secondary @click="actionCommand">{{ t('common.ssh') }}</n-button>
            <n-button secondary @click="actionEdit">{{ t('common.edit') }}</n-button>
            <n-button v-if="actionState !== 'running'" type="success" @click="actionStart">{{ t('common.start') }}</n-button>
            <n-button v-else type="warning" @click="actionStop">{{ t('common.stop') }}</n-button>
          </n-space>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showCommandModal" :title="t('tunnels.commandTitle', { name: commandTunnelLabel })" preset="dialog" style="width:720px">
      <n-space vertical :size="12" style="margin-top:8px">
        <n-text depth="3">{{ t('tunnels.commandHelp') }}</n-text>
        <n-text v-if="commandLoading" depth="3">{{ t('common.loading') }}</n-text>
        <n-input v-else :value="commandValue" type="textarea" :rows="6" readonly />
      </n-space>
      <template #action>
        <n-space justify="end"><n-button @click="showCommandModal = false">{{ t('common.close') }}</n-button></n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: #fff; border-bottom: 1px solid #e2e8f0; flex-shrink: 0; gap: 12px; }
.page-title { font-size: 14px; font-weight: 600; color: #1e293b; }
.page-body { flex: 1; overflow: auto; padding: 20px; }
.content-card { border-radius: 12px; }
.direction-help { display: flex; flex-direction: column; gap: 8px; font-size: 12px; color: #475569; }
.direction-help-row { display: flex; gap: 8px; align-items: flex-start; }
.direction-help-tag { min-width: 48px; display: inline-flex; justify-content: center; padding: 2px 8px; border-radius: 999px; background: #dbeafe; color: #1d4ed8; font-size: 11px; font-weight: 600; }
.direction-help-tag--target { background: #ede9fe; color: #6d28d9; }
.action-summary { display: flex; flex-direction: column; gap: 12px; }
.action-summary-row { display: flex; justify-content: space-between; gap: 16px; }
.action-summary-label { color: #64748b; font-size: 12px; }
.action-summary-value { display: inline-flex; align-items: center; gap: 8px; text-align: right; }
.action-summary-value.mono { font-family: 'SF Mono', 'Fira Code', monospace; font-size: 12px; }
.action-summary-error { color: #dc2626; font-size: 12px; max-width: 220px; }
.dir-chip { display: inline-flex; padding: 2px 8px; border-radius: 999px; font-family: 'SF Mono', 'Fira Code', monospace; font-size: 11px; font-weight: 700; }
.dir-chip.local { background: #dbeafe; color: #1d4ed8; }
.dir-chip.remote { background: #fce7f3; color: #9d174d; }
</style>
