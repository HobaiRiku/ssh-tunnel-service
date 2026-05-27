<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import {
  NDataTable, NButton, NModal, NForm, NFormItem, NInput, NInputNumber,
  NSpace, NAlert, NPopconfirm, useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRemotesStore } from '@/stores/remotes'
import type { Remote } from '@/api/client'

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
  } catch (e: any) {
    message.error(e.message)
  }
}

async function doDelete(id: string) {
  try {
    await store.deleteRemote(id)
    message.success('Remote deleted')
  } catch (e: any) {
    message.error(e.message)
  }
}

const columns: DataTableColumns<Remote> = [
  { title: 'ID', key: 'id', width: 120 },
  { title: 'Name', key: 'name' },
  { title: 'Host', key: 'host' },
  { title: 'Port', key: 'port', width: 80 },
  { title: 'User', key: 'user', width: 100 },
  { title: 'Description', key: 'description' },
  {
    title: 'Actions',
    key: 'actions',
    width: 160,
    render: (row) => h(NSpace, {}, {
      default: () => [
        h(NButton, { size: 'tiny', onClick: () => openEdit(row) }, { default: () => 'Edit' }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error' }, { default: () => 'Delete' }),
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
    <div class="toolbar">
      <h2>Remotes</h2>
      <n-button type="primary" size="small" @click="openAdd">+ Add Remote</n-button>
    </div>
    <n-alert v-if="store.error" type="error" :title="store.error" style="margin: 12px" />
    <div class="table-wrap">
      <n-data-table
        :columns="columns"
        :data="store.remotes"
        :loading="store.loading"
        :bordered="false"
        size="small"
      />
    </div>

    <n-modal v-model:show="showModal" :title="editingId ? 'Edit Remote' : 'Add Remote'" preset="dialog" style="width: 520px">
      <n-form label-placement="left" label-width="110">
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
          <n-input-number v-model:value="form.port" :min="1" :max="65535" />
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
.toolbar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px; border-bottom: 1px solid #e5e7eb;
}
.toolbar h2 { font-size: 16px; font-weight: 600; }
.table-wrap { padding: 16px; flex: 1; overflow: auto; }
</style>
