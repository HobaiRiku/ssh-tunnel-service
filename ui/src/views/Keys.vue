<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSpace,
  NText,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { getErrorMessage, type SSHKey, type SSHKeyPayload } from '@/api/client'
import { useKeysStore } from '@/stores/keys'
import { useI18n } from '@/i18n'

const keyStore = useKeysStore()
const message = useMessage()
const { t } = useI18n()

const showModal = ref(false)
const editingId = ref<string | null>(null)
const form = ref<SSHKeyPayload>({ id: '', name: '', file_name: '', private_key: '', description: '' })

function resetForm() {
  form.value = { id: '', name: '', file_name: '', private_key: '', description: '' }
}

function openAdd() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEdit(row: SSHKey) {
  editingId.value = row.id
  form.value = { name: row.name, file_name: row.file, private_key: '', description: row.description }
  showModal.value = true
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  form.value.file_name = file.name
  form.value.private_key = await file.text()
}

async function submitForm() {
  try {
    if (editingId.value) {
      await keyStore.updateKey(editingId.value, form.value)
      message.success(t('keys.updated'))
    } else {
      await keyStore.addKey(form.value)
      message.success(t('keys.added'))
    }
    showModal.value = false
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doDelete(id: string) {
  try {
    await keyStore.deleteKey(id)
    message.success(t('keys.deleted'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

const columns = computed<DataTableColumns<SSHKey>>(() => [
  { title: t('keys.columns.name'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('keys.columns.file'), key: 'file', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, row.file) },
  { title: t('keys.columns.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: t('keys.columns.actions'),
    key: 'actions',
    width: 150,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => t('common.delete') }),
          default: () => t('keys.deleteConfirm'),
        }),
      ],
    }),
  },
])

onMounted(() => {
  void keyStore.fetchKeys()
})
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('keys.title') }}</span>
      <n-button type="primary" size="small" @click="openAdd">{{ t('common.add') }}</n-button>
    </div>

    <div class="page-body">
      <n-alert v-if="keyStore.error" type="error" :title="keyStore.error" style="margin-bottom:16px" />
      <n-card :bordered="false" style="border-radius:10px;box-shadow:0 1px 6px rgba(0,0,0,0.06)">
        <n-data-table
          :columns="columns"
          :data="keyStore.keys"
          :loading="keyStore.loading"
          :bordered="false"
          size="small"
          :row-key="(row: SSHKey) => row.id"
        />
      </n-card>
    </div>

    <n-modal v-model:show="showModal" :title="editingId ? t('keys.editTitle') : t('keys.addTitle')" preset="dialog" style="width:620px">
      <n-form label-placement="left" label-width="110" style="margin-top:8px">
        <n-form-item v-if="!editingId" :label="t('keys.fields.id')">
          <n-input v-model:value="form.id" placeholder="e.g. deploy-key" />
        </n-form-item>
        <n-form-item :label="t('keys.fields.name')">
          <n-input v-model:value="form.name" :placeholder="t('keys.fields.name')" />
        </n-form-item>
        <n-form-item :label="t('keys.fields.fileName')">
          <n-input v-model:value="form.file_name" placeholder="id_ed25519" />
        </n-form-item>
        <n-form-item :label="t('keys.fields.privateKey')">
          <n-input v-model:value="form.private_key" type="textarea" :rows="8" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" />
        </n-form-item>
        <n-form-item :label="t('keys.fields.upload')">
          <div class="upload-box">
            <input type="file" @change="onFileChange" />
            <n-text depth="3">{{ t(editingId ? 'keys.replaceHint' : 'keys.uploadHint') }}</n-text>
          </div>
        </n-form-item>
        <n-form-item :label="t('keys.fields.description')">
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
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: #fff; border-bottom: 1px solid #e2e8f0; flex-shrink: 0; }
.page-title { font-size: 14px; font-weight: 600; color: #1e293b; }
.page-body { flex: 1; overflow: auto; padding: 20px; }
.upload-box { display: flex; flex-direction: column; gap: 8px; width: 100%; }
</style>
