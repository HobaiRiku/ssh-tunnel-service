<script setup lang="ts">
import { computed, h, onMounted, reactive, ref, watch } from 'vue'
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
import { copyText } from '@/clipboard'
import { validateName } from '@/validation'
import { useKeysStore } from '@/stores/keys'
import { useI18n } from '@/i18n'

const keyStore = useKeysStore()
const message = useMessage()
const { t } = useI18n()

const showModal = ref(false)
const showPublic = ref(false)
const publicKeyRow = ref<SSHKey | null>(null)
const editingName = ref<string | null>(null)
const submitted = ref(false)
const errors = reactive({ name: '' })

function emptyKey(): SSHKeyPayload {
  return { name: '', file_name: '', private_key: '', description: '' }
}
const form = ref<SSHKeyPayload>(emptyKey())

function runValidation(): boolean {
  errors.name = validateName(form.value.name, t) ?? ''
  return !errors.name
}

watch(form, () => {
  if (submitted.value) runValidation()
}, { deep: true })

function feedback(): string {
  return submitted.value ? errors.name : ''
}
function status(): 'error' | undefined {
  return submitted.value && errors.name ? 'error' : undefined
}

function openAdd() {
  editingName.value = null
  form.value = emptyKey()
  submitted.value = false
  errors.name = ''
  showModal.value = true
}

function openEdit(row: SSHKey) {
  editingName.value = row.name
  form.value = { name: row.name, file_name: row.file, private_key: '', description: row.description }
  submitted.value = false
  errors.name = ''
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
  submitted.value = true
  if (!runValidation()) {
    message.error(t('validation.fixErrors'))
    return
  }
  try {
    if (editingName.value) {
      await keyStore.updateKey(editingName.value, form.value)
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

async function doDelete(name: string) {
  try {
    await keyStore.deleteKey(name)
    message.success(t('keys.deleted'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function copyName(name: string) {
  const ok = await copyText(name)
  if (ok) message.success(t('common.copied'))
  else message.error(t('common.copyFailed'))
}

async function copyPublic(row: SSHKey) {
  if (!row.public_key) {
    message.error(t('keys.noPublicKey'))
    return
  }
  const ok = await copyText(row.public_key)
  if (ok) message.success(t('common.copied'))
  else message.error(t('common.copyFailed'))
}

function openPublic(row: SSHKey) {
  if (!row.public_key) {
    message.error(t('keys.noPublicKey'))
    return
  }
  publicKeyRow.value = row
  showPublic.value = true
}

const publicCopyCmd = computed(() => {
  const row = publicKeyRow.value
  if (!row) return ''
  return `ssh-tunnel key pub ${row.name} | ssh <user>@<host> 'cat >> ~/.ssh/authorized_keys'`
})

const columns = computed<DataTableColumns<SSHKey>>(() => [
  { title: t('keys.columns.name'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('keys.columns.file'), key: 'file', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, row.file) },
  { title: t('keys.columns.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: t('keys.columns.actions'),
    key: 'actions',
    width: 320,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
        h(NButton, { size: 'tiny', tertiary: true, onClick: () => { void copyName(row.name) } }, { default: () => t('common.copyName') }),
        h(NButton, { size: 'tiny', tertiary: true, type: 'primary', disabled: !row.public_key, onClick: () => openPublic(row) }, { default: () => t('keys.viewPublic') }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.name) }, {
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
          :row-key="(row: SSHKey) => row.name"
        />
      </n-card>
    </div>

    <n-modal v-model:show="showPublic" :title="t('keys.publicTitle')" preset="dialog" style="width:640px">
      <div v-if="publicKeyRow" class="pub-box">
        <n-alert type="info" :show-icon="true" :bordered="false">
          {{ t('keys.publicHint') }}
        </n-alert>
        <n-input
          :value="publicKeyRow.public_key"
          type="textarea"
          readonly
          :autosize="{ minRows: 2, maxRows: 4 }"
          class="pub-text"
        />
        <n-button size="small" type="primary" @click="() => publicKeyRow && copyPublic(publicKeyRow)">
          {{ t('keys.copyPublic') }}
        </n-button>
        <n-text depth="3" style="font-size:12px">{{ t('keys.publicCopyCmd') }}</n-text>
        <n-input :value="publicCopyCmd" type="textarea" readonly :autosize="{ minRows: 1, maxRows: 3 }" class="pub-text" />
      </div>
      <template #action>
        <n-button @click="showPublic = false">{{ t('common.cancel') }}</n-button>
      </template>
    </n-modal>

    <n-modal v-model:show="showModal" :title="editingName ? t('keys.editTitle') : t('keys.addTitle')" preset="dialog" style="width:620px">
      <n-form label-placement="left" label-width="110" style="margin-top:8px">
        <n-form-item :label="t('keys.fields.name')" :validation-status="status()" :feedback="feedback()">
          <n-input v-model:value="form.name" placeholder="e.g. deploy-key" />
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
            <n-text depth="3">{{ t(editingName ? 'keys.replaceHint' : 'keys.uploadHint') }}</n-text>
          </div>
        </n-form-item>
        <n-form-item :label="t('keys.fields.description')">
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
  </div>
</template>

<style scoped>
.page { display: flex; flex-direction: column; height: 100%; }
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: #fff; border-bottom: 1px solid #e2e8f0; flex-shrink: 0; }
.page-title { font-size: 14px; font-weight: 600; color: #1e293b; }
.page-body { flex: 1; overflow: auto; padding: 20px; }
.upload-box { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.pub-box { display: flex; flex-direction: column; gap: 12px; margin-top: 8px; }
.pub-box :deep(.pub-text textarea) { font-family: monospace; font-size: 12px; }
</style>
