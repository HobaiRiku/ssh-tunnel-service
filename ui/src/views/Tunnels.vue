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
import type { DataTableColumns } from 'naive-ui'
import { api, type Tunnel, type TunnelStatus } from '@/api/client'
import { useRemotesStore } from '@/stores/remotes'
import { useTunnelsStore } from '@/stores/tunnels'
import TunnelTopologyView from '@/components/TunnelTopologyView.vue'

const tunnelStore = useTunnelsStore()
const remoteStore = useRemotesStore()
const message = useMessage()

const showModal = ref(false)
const showCommandModal = ref(false)
const showActionModal = ref(false)
const commandLoading = ref(false)
const commandTunnelLabel = ref('')
const commandValue = ref('')
const editingId = ref<string | null>(null)
const viewMode = ref<'topology' | 'table'>('topology')
const activeTunnel = ref<TunnelStatus | null>(null)
const form = ref<Tunnel>({
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
})

const remoteOptions = computed(() =>
  remoteStore.remotes.map((remote) => ({ label: `${remote.name} (${remote.host})`, value: remote.id })),
)

type DirectionMeta = {
  value: '-L' | '-R'
  code: string
  title: string
  summary: string
  bindMeaning: string
  targetMeaning: string
}

const directionMeta: Record<'-L' | '-R', DirectionMeta> = {
  '-L': {
    value: '-L',
    code: '-L',
    title: 'Local forward',
    summary: 'Access a remote service from this machine.',
    bindMeaning: 'Bind = address/port THIS machine listens on (usually 127.0.0.1).',
    targetMeaning: 'Target = host:port the REMOTE will connect to (often localhost on the remote).',
  },
  '-R': {
    value: '-R',
    code: '-R',
    title: 'Remote forward',
    summary: 'Expose a local service on the remote machine.',
    bindMeaning: 'Bind = address/port the REMOTE listens on (use 0.0.0.0 to expose beyond loopback; requires GatewayPorts=yes on sshd).',
    targetMeaning: 'Target = host:port THIS machine will connect to (often 127.0.0.1 here).',
  },
}

const dirOptions = [directionMeta['-L'], directionMeta['-R']].map((meta) => ({
  value: meta.value,
  label: `${meta.code}  ${meta.title} — ${meta.summary}`,
  meta,
}))

function renderDirectionLabel(option: Record<string, unknown>) {
  const meta = option.meta as DirectionMeta | undefined
  if (!meta) return String(option.label ?? '')
  return h(
    'div',
    { style: 'display:flex;flex-direction:column;gap:2px;line-height:1.4;padding:2px 0' },
    [
      h(
        'div',
        { style: 'font-family:monospace;font-weight:600;font-size:13px;color:#1e293b' },
        `${meta.code}  ${meta.title}`,
      ),
      h(
        'div',
        { style: 'font-size:12px;color:#64748b;white-space:normal' },
        meta.summary,
      ),
    ],
  )
}

const selectedDirection = computed<DirectionMeta>(() => directionMeta[form.value.direction] ?? directionMeta['-L'])

const stateType: Record<string, 'success' | 'default' | 'error'> = {
  running: 'success',
  stopped: 'default',
  error: 'error',
}

const remoteNameMap = computed(() => {
  const names: Record<string, string> = {}
  for (const remote of remoteStore.remotes) names[remote.id] = remote.name
  return names
})

function resetForm() {
  form.value = {
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

function openAdd() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEdit(row: TunnelStatus) {
  editingId.value = row.id
  form.value = { ...row }
  showModal.value = true
}

async function refresh() {
  await Promise.all([tunnelStore.fetchTunnels(), remoteStore.fetchRemotes()])
}

async function submitForm() {
  try {
    if (editingId.value) {
      await tunnelStore.updateTunnel(editingId.value, form.value)
      message.success('Tunnel updated')
    } else {
      await tunnelStore.addTunnel(form.value)
      message.success('Tunnel added')
    }
    showModal.value = false
  } catch (error: any) {
    message.error(error.message)
  }
}

async function doDelete(id: string) {
  try {
    await tunnelStore.deleteTunnel(id)
    message.success('Tunnel deleted')
  } catch (error: any) {
    message.error(error.message)
  }
}

async function doStart(id: string) {
  try {
    await tunnelStore.startTunnel(id)
    message.info('Start requested')
  } catch (error: any) {
    message.error(error.message)
  }
}

async function doStop(id: string) {
  try {
    await tunnelStore.stopTunnel(id)
    message.success('Tunnel stopped')
  } catch (error: any) {
    message.error(error.message)
  }
}

function openActions(tunnel: TunnelStatus) {
  activeTunnel.value = tunnel
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

function actionCommand() {
  if (!activeTunnel.value) return
  const tunnel = activeTunnel.value
  closeActions()
  openCommand(tunnel)
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
  } catch (error: any) {
    showCommandModal.value = false
    message.error(error.message)
  } finally {
    commandLoading.value = false
  }
}

const columns: DataTableColumns<TunnelStatus> = [
  { title: 'Name', key: 'name', ellipsis: { tooltip: true } },
  {
    title: 'Remote',
    key: 'remote_id',
    width: 140,
    render: (row) => remoteNameMap.value[row.remote_id] || row.remote_id,
  },
  {
    title: 'Direction',
    key: 'direction',
    width: 80,
    render: (row) =>
      h(
        'span',
        {
          style: `font-family:monospace;font-size:11px;padding:2px 6px;border-radius:4px;background:${row.direction === '-L' ? '#dbeafe' : '#fce7f3'};color:${row.direction === '-L' ? '#1d4ed8' : '#9d174d'}`,
        },
        row.direction,
      ),
  },
  {
    title: 'Bind',
    key: 'bind',
    render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.bind_address}:${row.bind_port}`),
  },
  {
    title: 'Target',
    key: 'target',
    render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, `${row.target_host}:${row.target_port}`),
  },
  {
    title: 'State',
    key: 'state',
    width: 90,
    render: (row) => h(NTag, { type: stateType[row.state] || 'default', size: 'small', round: true }, { default: () => row.state }),
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 270,
    render: (row) =>
      h(NSpace, { size: 'small' }, {
        default: () => [
          row.state !== 'running'
            ? h(NButton, { size: 'tiny', type: 'success', onClick: () => doStart(row.id) }, { default: () => 'Start' })
            : h(NButton, { size: 'tiny', type: 'warning', onClick: () => doStop(row.id) }, { default: () => 'Stop' }),
          h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => 'Edit' }),
          h(NButton, { size: 'tiny', tertiary: true, onClick: () => openCommand(row) }, { default: () => 'SSH' }),
          h(
            NPopconfirm,
            { onPositiveClick: () => doDelete(row.id) },
            {
              trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => 'Delete' }),
              default: () => 'Delete this tunnel?',
            },
          ),
        ],
      }),
  },
]

onMounted(refresh)
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">Tunnels</span>
      <n-space align="center">
        <n-button-group>
          <n-button :type="viewMode === 'topology' ? 'primary' : 'default'" @click="viewMode = 'topology'">Topology</n-button>
          <n-button :type="viewMode === 'table' ? 'primary' : 'default'" @click="viewMode = 'table'">Table</n-button>
        </n-button-group>
        <n-button secondary :loading="tunnelStore.loading || remoteStore.loading" @click="refresh">Refresh</n-button>
        <n-button type="primary" @click="openAdd">Add Tunnel</n-button>
      </n-space>
    </div>

    <div class="page-body">
      <n-alert v-if="tunnelStore.error" type="error" :title="tunnelStore.error" style="margin-bottom: 16px" />
      <n-card :bordered="false" class="content-card">
        <TunnelTopologyView
          v-if="viewMode === 'topology'"
          :tunnels="tunnelStore.tunnels"
          :remotes="remoteStore.remotes"
          :loading="tunnelStore.loading || remoteStore.loading"
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

    <n-modal
      v-model:show="showModal"
      :title="editingId ? 'Edit Tunnel' : 'Add Tunnel'"
      preset="dialog"
      style="width: 560px"
    >
      <n-form label-placement="left" label-width="120" style="margin-top: 8px">
        <n-form-item v-if="!editingId" label="ID">
          <n-input v-model:value="form.id" placeholder="e.g. db-forward" />
        </n-form-item>
        <n-form-item label="Name">
          <n-input v-model:value="form.name" placeholder="Display name" />
        </n-form-item>
        <n-form-item label="Remote">
          <n-select v-model:value="form.remote_id" :options="remoteOptions" placeholder="Select remote" />
        </n-form-item>
        <n-form-item label="Direction">
          <n-select
            v-model:value="form.direction"
            :options="dirOptions"
            :render-label="renderDirectionLabel"
          />
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
        <n-form-item label="Bind Address">
          <n-input v-model:value="form.bind_address" :placeholder="form.direction === '-R' ? '127.0.0.1 or 0.0.0.0 (on remote)' : '127.0.0.1 (on this machine)'" />
        </n-form-item>
        <n-form-item label="Bind Port">
          <n-input-number v-model:value="form.bind_port" :min="1" :max="65535" style="width: 100%" />
        </n-form-item>
        <n-form-item label="Target Host">
          <n-input v-model:value="form.target_host" :placeholder="form.direction === '-R' ? 'localhost (resolved on this machine)' : 'localhost (resolved on remote)'" />
        </n-form-item>
        <n-form-item label="Target Port">
          <n-input-number v-model:value="form.target_port" :min="1" :max="65535" style="width: 100%" />
        </n-form-item>
        <n-form-item label="Auto Start">
          <n-switch v-model:value="form.auto_start" />
        </n-form-item>
        <n-form-item label="Description">
          <n-input v-model:value="form.description" type="textarea" :rows="2" />
        </n-form-item>
      </n-form>
      <template #action>
        <n-space justify="end">
          <n-button @click="showModal = false">Cancel</n-button>
          <n-button type="primary" @click="submitForm">{{ editingId ? 'Save' : 'Add' }}</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showActionModal"
      :title="activeTunnel ? `Tunnel · ${activeTunnel.name || activeTunnel.id}` : 'Tunnel'"
      preset="dialog"
      style="width: 460px"
    >
      <div v-if="activeTunnel" class="action-summary">
        <div class="action-summary-row">
          <span class="action-summary-label">Remote</span>
          <span class="action-summary-value">{{ remoteNameMap[activeTunnel.remote_id] || activeTunnel.remote_id }}</span>
        </div>
        <div class="action-summary-row">
          <span class="action-summary-label">Direction</span>
          <span class="action-summary-value">
            <span class="dir-chip" :class="activeTunnel.direction === '-L' ? 'local' : 'remote'">{{ activeTunnel.direction }}</span>
          </span>
        </div>
        <div class="action-summary-row">
          <span class="action-summary-label">Bind</span>
          <span class="action-summary-value mono">{{ activeTunnel.bind_address }}:{{ activeTunnel.bind_port }}</span>
        </div>
        <div class="action-summary-row">
          <span class="action-summary-label">Target</span>
          <span class="action-summary-value mono">{{ activeTunnel.target_host }}:{{ activeTunnel.target_port }}</span>
        </div>
        <div class="action-summary-row">
          <span class="action-summary-label">State</span>
          <span class="action-summary-value">
            <n-tag :type="stateType[activeTunnel.state] || 'default'" size="small" round>{{ activeTunnel.state }}</n-tag>
            <span v-if="activeTunnel.error" class="action-summary-error">{{ activeTunnel.error }}</span>
          </span>
        </div>
      </div>
      <template #action>
        <n-space justify="space-between" style="width:100%">
          <n-popconfirm @positive-click="actionDelete">
            <template #trigger>
              <n-button type="error" ghost>Delete</n-button>
            </template>
            Delete this tunnel?
          </n-popconfirm>
          <n-space>
            <n-button @click="closeActions">Close</n-button>
            <n-button secondary @click="actionCommand">SSH Command</n-button>
            <n-button secondary @click="actionEdit">Edit</n-button>
            <n-button
              v-if="actionState !== 'running'"
              type="success"
              @click="actionStart"
            >Start</n-button>
            <n-button
              v-else
              type="warning"
              @click="actionStop"
            >Stop</n-button>
          </n-space>
        </n-space>
      </template>
    </n-modal>

    <n-modal
      v-model:show="showCommandModal"
      :title="`Equivalent SSH Command · ${commandTunnelLabel}`"
      preset="dialog"
      style="width: 720px"
    >
      <n-space vertical :size="12" style="margin-top: 8px">
        <n-text depth="3">SSH arguments the service will pass to <code>ssh</code> for this tunnel. Note: POSIX shell rendering — may not be directly runnable on non-POSIX shells.</n-text>
        <n-text v-if="commandLoading" depth="3">Loading command…</n-text>
        <n-input v-else :value="commandValue" type="textarea" :rows="6" readonly />
      </n-space>
      <template #action>
        <n-space justify="end">
          <n-button @click="showCommandModal = false">Close</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 20px;
  background: #fff;
  border-bottom: 1px solid #e2e8f0;
  flex-shrink: 0;
  gap: 12px;
}

.page-title {
  font-size: 14px;
  font-weight: 600;
  color: #1e293b;
}

.page-body {
  flex: 1;
  overflow: auto;
  padding: 20px;
}

.content-card {
  border-radius: 10px;
  box-shadow: 0 1px 6px rgba(0, 0, 0, 0.06);
  min-height: calc(100vh - 160px);
}

.direction-help {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 14px;
  margin: 4px 0 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.55;
  color: #475569;
  width: 100%;
}

.direction-help-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.direction-help-tag {
  flex-shrink: 0;
  display: inline-block;
  min-width: 52px;
  text-align: center;
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.2px;
  color: #1d4ed8;
  background: #dbeafe;
  padding: 2px 6px;
  border-radius: 4px;
  line-height: 1.5;
}

.direction-help-tag--target {
  color: #9d174d;
  background: #fce7f3;
}

.action-summary {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 6px 0 4px;
}

.action-summary-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.action-summary-label {
  flex-shrink: 0;
  width: 80px;
  font-size: 12px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.4px;
}

.action-summary-value {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #1e293b;
  flex: 1;
  min-width: 0;
  flex-wrap: wrap;
}

.action-summary-value.mono {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 12.5px;
}

.action-summary-error {
  font-size: 11.5px;
  color: #b91c1c;
  word-break: break-word;
}

.dir-chip {
  font-family: monospace;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 7px;
  border-radius: 4px;
}

.dir-chip.local {
  background: #dbeafe;
  color: #1d4ed8;
}

.dir-chip.remote {
  background: #fce7f3;
  color: #9d174d;
}
</style>
