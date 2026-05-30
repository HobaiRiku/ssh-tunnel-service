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
  NInputNumber,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { getErrorMessage, type Remote } from '@/api/client'
import { useKeysStore } from '@/stores/keys'
import { useRemotesStore } from '@/stores/remotes'
import { useI18n } from '@/i18n'

const remoteStore = useRemotesStore()
const keyStore = useKeysStore()
const message = useMessage()
const { t } = useI18n()

const showModal = ref(false)
const editingId = ref<string | null>(null)
const form = ref<Remote>({ id: '', name: '', host: '', port: 22, user: '', key_id: '', description: '' })

const keyOptions = computed(() => [
  { label: t('common.systemDefault'), value: '' },
  ...keyStore.keys.map((key) => ({ label: key.name, value: key.id })),
])
const keyNameMap = computed(() => new Map(keyStore.keys.map((key) => [key.id, key.name] as const)))

function openAdd() {
  editingId.value = null
  form.value = { id: '', name: '', host: '', port: 22, user: '', key_id: '', description: '' }
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
      await remoteStore.updateRemote(editingId.value, form.value)
      message.success(t('remotes.updated'))
    } else {
      await remoteStore.addRemote(form.value)
      message.success(t('remotes.added'))
    }
    showModal.value = false
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function doDelete(id: string) {
  try {
    await remoteStore.deleteRemote(id)
    message.success(t('remotes.deleted'))
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

const columns = computed<DataTableColumns<Remote>>(() => [
  { title: t('remotes.columns.name'), key: 'name', ellipsis: { tooltip: true } },
  { title: t('remotes.columns.host'), key: 'host', render: (row) => h('span', { style: 'font-family:monospace;font-size:12px' }, row.host) },
  { title: t('remotes.columns.port'), key: 'port', width: 80 },
  { title: t('remotes.columns.user'), key: 'user', width: 110 },
  { title: t('remotes.columns.key'), key: 'key_id', width: 140, render: (row) => keyNameMap.value.get(row.key_id) ?? t('common.systemDefault') },
  { title: t('remotes.columns.description'), key: 'description', ellipsis: { tooltip: true } },
  {
    title: t('remotes.columns.actions'),
    key: 'actions',
    width: 150,
    render: (row) => h(NSpace, { size: 'small' }, {
      default: () => [
        h(NButton, { size: 'tiny', secondary: true, onClick: () => openEdit(row) }, { default: () => t('common.edit') }),
        h(NPopconfirm, { onPositiveClick: () => doDelete(row.id) }, {
          trigger: () => h(NButton, { size: 'tiny', type: 'error', ghost: true }, { default: () => t('common.delete') }),
          default: () => t('remotes.deleteConfirm'),
        }),
      ],
    }),
  },
])

onMounted(async () => {
  await Promise.all([remoteStore.fetchRemotes(), keyStore.fetchKeys()])
})
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('remotes.title') }}</span>
      <n-button type="primary" size="small" @click="openAdd">{{ t('common.add') }}</n-button>
    </div>

    <div class="page-body">
      <n-alert v-if="remoteStore.error" type="error" :title="remoteStore.error" style="margin-bottom:16px" />
      <n-alert v-if="keyStore.error" type="error" :title="keyStore.error" style="margin-bottom:16px" />
      <n-card :bordered="false" style="border-radius:10px;box-shadow:0 1px 6px rgba(0,0,0,0.06)">
        <n-data-table
          :columns="columns"
          :data="remoteStore.remotes"
          :loading="remoteStore.loading || keyStore.loading"
          :bordered="false"
          size="small"
          :row-key="(row: Remote) => row.id"
        />
      </n-card>
    </div>

    <n-modal v-model:show="showModal" :title="editingId ? t('remotes.editTitle') : t('remotes.addTitle')" preset="dialog" style="width:540px">
      <n-form label-placement="left" label-width="110" style="margin-top:8px">
        <n-form-item v-if="!editingId" :label="t('remotes.fields.id')">
          <n-input v-model:value="form.id" placeholder="e.g. prod-server" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.name')">
          <n-input v-model:value="form.name" :placeholder="t('remotes.fields.name')" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.host')">
          <n-input v-model:value="form.host" placeholder="192.168.1.1" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.port')">
          <n-input-number v-model:value="form.port" :min="1" :max="65535" style="width:100%" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.user')">
          <n-input v-model:value="form.user" placeholder="ubuntu" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.key')">
          <n-select v-model:value="form.key_id" :options="keyOptions" />
        </n-form-item>
        <n-form-item :label="t('remotes.fields.description')">
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
</style>
