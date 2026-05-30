<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSpace, NAlert, NPopconfirm, useMessage, NCard
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRemotesStore } from '@/stores/remotes'
import { getErrorMessage, type Remote } from '@/api/client'

const store = useRemotesStore()
const message = useMessage()

const showModal = ref(false)
const editingId = ref<string | null>(null)
const form = ref<Remote>({ id: '', name: '', host: '', port: 22, user: '', description: '' })

function openAdd() {
  editingId.value = null
  form.value = { id: '', name: '', host: '', port: 22, user: '', description: '' }
  showModal.value = true
}

function openEdit(row: Remote) {
  editingId.value = row.id
  form.value = { ...row }
  showModal.value = true
}

async function submitForm() {
  try {
    if (editingId.value) {
      await store.updateRemote(editingId.value, form.value)
      message.success('Remote updated')
    } else {
      await store.addRemote(form.value)
      message.success('Remote added')
    }
    showModal.value = false
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doDelete(id: string) {
  try {
    await store.deleteRemote(id)
    message.success('Remote deleted')
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

const columns: DataTableColumns<Remote> = [
  { title: 'Name', key: 'name', ellipsis: { tooltip: true } },
  {
    title: 'Host',
    key: 'host',
    render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, row.host)
  },
  { title: 'Port', key: 'port', width: 80 },
  { title: 'User', key: 'user', width: 110 },
  {
    title: 'Description',
    key: 'description',
    ellipsis: { tooltip: true }
  },
  {
    title: 'Actions',
    key: 'actions',
    width: 140,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => 'Edit' }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => 'Delete' }),
          default: () => 'Delete this remote?'
        })
      ]
    })
  }
]

onMounted(() => store.fetchRemotes())
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">Remotes</span>
      <n-button type="primary" size="small" @click="openAdd">Add Remote</n-button>
    </div>

    <div class="page-body">
      <n-alert v-if="store.error" type="error" :title="store.error" style="margin-bottom:16px" />
      <n-card :bordered="false" style="border-radius:10px;box-shadow:0 1px 6px rgba(0,0,0,0.06)">
        <n-data-table
          :columns="columns"
          :data="store.remotes"
          :loading="store.loading"
          :bordered="false"
          size="small"
          :row-key="(row: Remote) => row.id"
        />
      </n-card>
    </div>

    <n-modal
      v-model:show="showModal"
      :title="editingId ? 'Edit Remote' : 'Add Remote'"
      preset="dialog"
      style="width:520px"
    >
      <n-form label-placement="left" label-width="110" style="margin-top:8px">
        <n-form-item label="ID" v-if="!editingId">
          <n-input v-model:value="form.id" placeholder="e.g. prod-server" />
        </n-form-item>
        <n-form-item label="Name">
          <n-input v-model:value="form.name" placeholder="Display name" />
        </n-form-item>
        <n-form-item label="Host">
          <n-input v-model:value="form.host" placeholder="192.168.1.1" />
        </n-form-item>
        <n-form-item label="Port">
          <n-input-number v-model:value="form.port" :min="1" :max="65535" style="width:100%" />
        </n-form-item>
        <n-form-item label="User">
          <n-input v-model:value="form.user" placeholder="ubuntu" />
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

.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 20px;
  background: #fff;
  border-bottom: 1px solid #e2e8f0;
  flex-shrink: 0;
}

.page-title { font-size: 14px; font-weight: 600; color: #1e293b; }

.page-body {
  flex: 1;
  overflow: auto;
  padding: 20px;
}
</style>
