<script setup lang="ts">
import { computed, h, nextTick, onMounted, ref } from 'vue'
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
import type { DataTableColumns } from 'naive-ui'
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
    auto_start: true,
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
    ssh_options: Array.isArray(tunnel.ssh_options) ? [...tunnel.ssh_options] : [],
    auto_start: tunnel.auto_start,
    description: tunnel.description,
  }
}

const form = ref<Tunnel>(createDefaultTunnel())
const remoteOptions = computed(() => remoteStore.remotes.map((remote) => ({ label: `${remote.name} (${remote.host})`, value: remote.id })))
const remoteNameMap = computed(() => new Map(remoteStore.remotes.map((remote) => [remote.id, remote.name] as const)))

type TunnelDirection = Tunnel['direction']
type DirectionMeta = { value: TunnelDirection; code: string; title: string; summary: string; bindMeaning: string; targetMeaning: string }

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

const directionList = computed<DirectionMeta[]>(() => [directionMeta.value['-L'], directionMeta.value['-R']])
const selectedDirection = computed(() => directionMeta.value[form.value.direction] ?? directionMeta.value['-L'])
const stateType: { [state in TunnelStatus['state']]: 'success' | 'default' | 'error' } = { running: 'success', stopped: 'default', error: 'error' }

function resetForm() {
  form.value = createDefaultTunnel()
}

async function openAdd() {
  showModal.value = false
  editingId.value = null
  resetForm()
  await nextTick()
  showModal.value = true
}

async function openEdit(row: TunnelStatus) {
  showModal.value = false
  editingId.value = row.id
  form.value = toTunnelForm(row)
  await nextTick()
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

async function actionEdit() {
  if (!activeTunnel.value) return
  const tunnel = activeTunnel.value
  closeActions()
  await nextTick()
  await new Promise<void>((resolve) => window.setTimeout(resolve, 0))
  await openEdit(tunnel)
}

async function actionCommand() {
  if (!activeTunnel.value) return
  const tunnel = activeTunnel.value
  closeActions()
  await nextTick()
  await new Promise<void>((resolve) => window.setTimeout(resolve, 0))
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
        h(NButton, { size: 'tiny', secondary: true, onClick: () => { void openEdit(row) } }, { default: () => t('common.edit') }),
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
        <n-button type="primary" @click="void openAdd()">{{ t('common.add') }}</n-button>
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

    <n-modal v-model:show="showModal" :title="editingId ? t('tunnels.editTitle') : t('tunnels.addTitle')" preset="dialog" style="width:680px">
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
              <span class="direction-help-tag">Bind</span>
              <span>{{ selectedDirection.bindMeaning }}</span>
            </div>
            <div class="direction-help-row">
              <span class="direction-help-tag direction-help-tag--target">Target</span>
              <span>{{ selectedDirection.targetMeaning }}</span>
            </div>
          </div>
        </n-form-item>
        <n-form-item :label="t('tunnels.columns.bind')">
          <div class="addr-row">
            <n-input v-model:value="form.bind_address" class="addr-row-host" />
            <span class="addr-row-sep">:</span>
            <n-input-number v-model:value="form.bind_port" :min="1" :max="65535" :show-button="false" class="addr-row-port" />
          </div>
        </n-form-item>
        <n-form-item :label="t('tunnels.columns.target')">
          <div class="addr-row">
            <n-input v-model:value="form.target_host" class="addr-row-host" />
            <span class="addr-row-sep">:</span>
            <n-input-number v-model:value="form.target_port" :min="1" :max="65535" :show-button="false" class="addr-row-port" />
          </div>
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
            <n-button secondary @click="void actionCommand()">{{ t('common.ssh') }}</n-button>
            <n-button secondary @click="void actionEdit()">{{ t('common.edit') }}</n-button>
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
  border: 1.5px solid #e2e8f0;
  border-radius: 10px;
  background: #fff;
  cursor: pointer;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
}

.direction-card:hover { border-color: #cbd5e1; }
.direction-card.active.local { border-color: #1d4ed8; background: #eff6ff; box-shadow: 0 0 0 3px rgba(29, 78, 216, 0.1); }
.direction-card.active.remote { border-color: #a21caf; background: #fdf4ff; box-shadow: 0 0 0 3px rgba(162, 28, 175, 0.1); }

.direction-card-input { position: absolute; opacity: 0; pointer-events: none; }

.direction-card-radio {
  width: 14px;
  height: 14px;
  margin-top: 2px;
  border-radius: 50%;
  border: 1.5px solid #cbd5e1;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: #fff;
}

.direction-card.active.local .direction-card-radio { border-color: #1d4ed8; }
.direction-card.active.remote .direction-card-radio { border-color: #a21caf; }

.direction-card.active .direction-card-radio::after {
  content: '';
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}
.direction-card.active.local .direction-card-radio::after { background: #1d4ed8; }
.direction-card.active.remote .direction-card-radio::after { background: #a21caf; }

.direction-card-body { display: flex; flex-direction: column; gap: 4px; line-height: 1.35; min-width: 0; }
.direction-card-head { display: flex; align-items: center; gap: 6px; }
.direction-card-code {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #334155;
}
.direction-card.local .direction-card-code { background: #dbeafe; color: #1d4ed8; }
.direction-card.remote .direction-card-code { background: #fce7f3; color: #9d174d; }
.direction-card-title { font-size: 13px; font-weight: 600; color: #1e293b; }
.direction-card-summary { font-size: 12px; color: #64748b; }

.addr-row { display: flex; align-items: center; gap: 6px; width: 100%; }
.addr-row-host { flex: 1; min-width: 0; }
.addr-row-sep { color: #94a3b8; font-weight: 600; }
.addr-row-port { width: 110px; flex-shrink: 0; }

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
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  font-size: 12px;
  color: #475569;
}
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
