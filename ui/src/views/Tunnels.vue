<script setup lang="ts">
import { ref, onMounted, h, computed } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSelect, NSwitch, NSpace, NAlert, NTag, NPopconfirm, useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useTunnelsStore } from '@/stores/tunnels'
import { useRemotesStore } from '@/stores/remotes'
import type { Tunnel, TunnelStatus } from '@/api/client'

const tunnelStore = useTunnelsStore()
const remoteStore = useRemotesStore()
const message = useMessage()

const showModal = ref(false)
const editingId = ref<string | null>(null)
const form = ref<Tunnel>({
  id: '', name: '', remote_id: '', direction: '-L',
  bind_address: '127.0.0.1', bind_port: 0,
  target_host: '', target_port: 0,
  ssh_options: [], auto_start: false, description: ''
})

const remoteOptions = computed(() =>
  remoteStore.remotes.map(r => ({ label: `${r.name} (${r.host})`, value: r.id }))
)

const dirOptions = [
  { label: '-L  Local forward (local:port → remote:target)', value: '-L' },
  { label: '-R  Remote forward (remote:port → local:target)', value: '-R' }
]

function openAdd() {
  editingId.value = null
  form.value = {
    id: '', name: '', remote_id: '', direction: '-L',
    bind_address: '127.0.0.1', bind_port: 0,
    target_host: '', target_port: 0,
    ssh_options: [], auto_start: false, description: ''
  }
  showModal.value = true
}

function openEdit(row: TunnelStatus) {
  editingId.value = row.id
  form.value = { ...row }
  showModal.value = true
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
  } catch (e: any) {
    message.error(e.message)
  }
}

async function doDelete(id: string) {
  try {
    await tunnelStore.deleteTunnel(id)
    message.success('Tunnel deleted')
  } catch (e: any) {
    message.error(e.message)
  }
}

async function doStart(id: string) {
  try {
    await tunnelStore.startTunnel(id)
    message.success('Tunnel started')
  } catch (e: any) {
    message.error(e.message)
  }
}

async function doStop(id: string) {
  try {
    await tunnelStore.stopTunnel(id)
    message.success('Tunnel stopped')
  } catch (e: any) {
    message.error(e.message)
  }
}

const stateType: Record<string, 'success' | 'default' | 'error'> = {
  running: 'success',
  stopped: 'default',
  error: 'error'
}

const columns: DataTableColumns<TunnelStatus> = [
  { title: 'ID', key: 'id', width: 120 },
  { title: 'Name', key: 'name' },
  { title: 'Dir', key: 'direction', width: 50 },
  { title: 'Remote', key: 'remote_id', width: 120 },
  {
    title: 'Bind',
    key: 'bind',
    render: (row) => `${row.bind_address}:${row.bind_port}`
  },
  {
    title: 'Target',
    key: 'target',
    render: (row) => `${row.target_host}:${row.target_port}`
  },
  {
    title: 'State',
    key: 'state',
    width: 90,
    render: (row) => h(NTag, { type: stateType[row.state] || 'default', size: 'small' }, { default: () => row.state })
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 220,
    render: (row) => h(NSpace, {}, {
      default: () => [
        row.state !== 'running'
          ? h(NButton, { size: 'tiny', type: 'success', onClick: () => doStart(row.id) }, { default: () => 'Start' })
          : h(NButton, { size: 'tiny', onClick: () => doStop(row.id) }, { default: () => 'Stop' }),
        h(NButton, { size: 'tiny', onClick: () => openEdit(row) }, { default: () => 'Edit' }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, { default: () => 'Delete' }),
          default: () => 'Delete this tunnel?'
        })
      ]
    })
  }
]

onMounted(() => {
  tunnelStore.fetchTunnels()
  remoteStore.fetchRemotes()
})
</script>

<template>
  <div class="page">
    <div class="toolbar">
      <h2>Tunnels</h2>
      <n-button type="primary" size="small" @click="openAdd">+ Add Tunnel</n-button>
    </div>
    <n-alert v-if="tunnelStore.error" type="error" :title="tunnelStore.error" style="margin: 12px" />
    <div class="table-wrap">
      <n-data-table
        :columns="columns"
        :data="tunnelStore.tunnels"
        :loading="tunnelStore.loading"
        :bordered="false"
        size="small"
      />
    </div>

    <n-modal v-model:show="showModal" :title="editingId ? 'Edit Tunnel' : 'Add Tunnel'" preset="dialog" style="width: 560px">
      <n-form label-placement="left" label-width="120">
        <n-form-item label="ID" v-if="!editingId">
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
          <n-input-number v-model:value="form.bind_port" :min="1" :max="65535" />
        </n-form-item>
        <n-form-item label="Target Host">
          <n-input v-model:value="form.target_host" placeholder="localhost" />
        </n-form-item>
        <n-form-item label="Target Port">
          <n-input-number v-model:value="form.target_port" :min="1" :max="65535" />
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
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px; border-bottom: 1px solid #e5e7eb;
}
.toolbar h2 { font-size: 16px; font-weight: 600; }
.table-wrap { padding: 16px; flex: 1; overflow: auto; }
</style>
