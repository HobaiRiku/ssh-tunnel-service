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
const commandLoading = ref(false)
const commandTunnelLabel = ref('')
const commandValue = ref('')
const editingId = ref<string | null>(null)
const viewMode = ref<'topology' | 'table'>('topology')
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

const dirOptions = [
  { label: '-L  Local forward (local:port → remote:target)', value: '-L' },
  { label: '-R  Remote forward (remote:port → local:target)', value: '-R' },
]

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
          @start="doStart"
          @stop="doStop"
          @edit="openEdit"
          @delete="doDelete"
          @command="openCommand"
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
          <n-select v-model:value="form.direction" :options="dirOptions" />
        </n-form-item>
        <n-form-item label="Bind Address">
          <n-input v-model:value="form.bind_address" placeholder="127.0.0.1" />
        </n-form-item>
        <n-form-item label="Bind Port">
          <n-input-number v-model:value="form.bind_port" :min="1" :max="65535" style="width: 100%" />
        </n-form-item>
        <n-form-item label="Target Host">
          <n-input v-model:value="form.target_host" placeholder="localhost" />
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
</style>
