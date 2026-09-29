<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSpace,
  NText,
  useDialog,
  useMessage,
} from 'naive-ui'
import type { DropdownOption } from 'naive-ui'
import { getErrorMessage, type SSHKey, type SSHKeyPayload } from '@/api/client'
import { copyText } from '@/clipboard'
import { validateName } from '@/validation'
import { useInstanceStore } from '@/stores/instance'
import { useKeysStore } from '@/stores/keys'
import { useRemotesStore } from '@/stores/remotes'
import { useI18n } from '@/i18n'

// Keys are identities the service logs in with. Each card shows where a key is
// used and hands out what a server needs to trust it (the public key, or a
// ready-made command that installs it).
const keyStore = useKeysStore()
const remoteStore = useRemotesStore()
const instance = useInstanceStore()
const message = useMessage()
const dialog = useDialog()
const { t } = useI18n()

const showModal = ref(false)
const editingName = ref<string | null>(null)
const submitted = ref(false)
const errors = reactive({ name: '' })
const source = ref<'paste' | 'file'>('paste')
const pickedFile = ref('')

function emptyKey(): SSHKeyPayload {
  return { name: '', file_name: '', private_key: '', description: '' }
}
const form = ref<SSHKeyPayload>(emptyKey())

// Servers that name the key explicitly; the system default additionally covers
// every unbound server when the service runs as a system service.
function usedBy(key: SSHKey): string[] {
  return remoteStore.remotes.filter((r) => r.key === key.name).map((r) => r.name)
}
const unboundServers = computed(() => remoteStore.remotes.filter((r) => !r.key).map((r) => r.name))

function authorizeCommand(key: SSHKey): string {
  const pub = key.public_key?.trim()
  if (!pub) return ''
  return `mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '${pub}' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`
}

function keyType(key: SSHKey): string {
  return key.public_key?.split(' ')[0]?.replace(/^ssh-/, '') ?? ''
}

function runValidation(): boolean {
  errors.name = validateName(form.value.name, t) ?? ''
  return !errors.name
}

watch(form, () => {
  if (submitted.value) runValidation()
}, { deep: true })

function openAdd() {
  editingName.value = null
  form.value = emptyKey()
  source.value = 'paste'
  pickedFile.value = ''
  submitted.value = false
  errors.name = ''
  showModal.value = true
}

function openEdit(row: SSHKey) {
  editingName.value = row.name
  form.value = { name: row.name, file_name: row.file, private_key: '', description: row.description }
  source.value = 'paste'
  pickedFile.value = ''
  submitted.value = false
  errors.name = ''
  showModal.value = true
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  pickedFile.value = file.name
  form.value.file_name = file.name
  form.value.private_key = await file.text()
  if (!form.value.name) form.value.name = file.name.replace(/\.(pem|key)$/i, '')
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
    await remoteStore.fetchRemotes()
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  }
}

async function copy(value: string, done: string) {
  if (!value) {
    message.error(t('keys.noPublicKey'))
    return
  }
  const ok = await copyText(value)
  if (ok) message.success(done)
  else message.error(t('common.copyFailed'))
}

function confirmDelete(key: SSHKey) {
  // The service refuses both of these; explain instead of failing.
  if (key.system_default) {
    dialog.info({ title: t('keysPage.cannotDelete'), content: t('keysPage.defaultBlocks'), positiveText: t('common.close') })
    return
  }
  const servers = usedBy(key)
  if (servers.length > 0) {
    dialog.info({ title: t('keysPage.cannotDelete'), content: t('keysPage.usedBlocks', { servers: servers.join('、') }), positiveText: t('common.close') })
    return
  }
  dialog.warning({
    title: t('keys.deleteConfirm'),
    content: t('keysPage.deleteBody', { name: key.name }),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await keyStore.deleteKey(key.name)
        message.success(t('keys.deleted'))
      } catch (error: unknown) {
        message.error(getErrorMessage(error))
      }
    },
  })
}

function confirmDefault(key: SSHKey) {
  dialog.warning({
    title: t('keysPage.makeDefaultTitle', { name: key.name }),
    content: t('keysPage.makeDefaultBody', { n: unboundServers.value.length }),
    positiveText: t('keysPage.makeDefault'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      try {
        await keyStore.setDefaultKey(key.name)
        message.success(t('keysPage.defaultChanged', { name: key.name }))
      } catch (error: unknown) {
        message.error(getErrorMessage(error))
      }
    },
  })
}

function menu(key: SSHKey): DropdownOption[] {
  return [
    ...(instance.isSystem && !key.system_default ? [{ key: 'default', label: t('keysPage.makeDefault') }] : []),
    { key: 'copyName', label: t('common.copyName') },
    { type: 'divider', key: 'd' },
    { key: 'delete', label: t('common.delete') },
  ]
}

async function onMenu(action: string, key: SSHKey) {
  if (action === 'default') return confirmDefault(key)
  if (action === 'copyName') return copy(key.name, t('common.copied'))
  if (action === 'delete') confirmDelete(key)
}

onMounted(() => {
  void instance.load()
  void keyStore.fetchKeys()
  void remoteStore.fetchRemotes()
})
</script>

<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="page-title">{{ t('keys.title') }}</span>
      <n-button type="primary" @click="openAdd">{{ t('keysPage.add') }}</n-button>
    </div>

    <div class="page-body">
      <n-alert v-if="keyStore.error" type="error" :title="keyStore.error" style="margin-bottom:16px" />
      <p class="intro">{{ instance.isSystem ? t('keysPage.introSystem') : t('keysPage.introUser') }}</p>

      <div v-if="!keyStore.loading && keyStore.keys.length === 0" class="empty">
        <h3>{{ t('keysPage.emptyTitle') }}</h3>
        <p>{{ t('keysPage.emptySub') }}</p>
        <n-button type="primary" @click="openAdd">{{ t('keysPage.add') }}</n-button>
      </div>

      <div v-else class="grid">
        <article v-for="key in keyStore.keys" :key="key.name" class="key" :class="{ primary: key.system_default }">
          <header class="head">
            <span class="icon" aria-hidden="true">
              <svg viewBox="0 0 20 20"><circle cx="7" cy="10" r="3.5" /><path d="M10.5 10H17m-2.5 0v2.5M16.5 10v2" /></svg>
            </span>
            <div class="titles">
              <h3 class="name">
                {{ key.name }}
                <span v-if="key.system_default" class="badge">{{ t('keysPage.defaultBadge') }}</span>
              </h3>
              <span class="meta">{{ [keyType(key), key.file].filter(Boolean).join(' · ') }}</span>
            </div>
            <n-dropdown trigger="click" :options="menu(key)" placement="bottom-end" @select="(a: string) => onMenu(a, key)">
              <n-button size="small" quaternary :aria-label="t('overview.more')">
                <svg class="more" viewBox="0 0 20 20" aria-hidden="true"><circle cx="4" cy="10" r="1.6" /><circle cx="10" cy="10" r="1.6" /><circle cx="16" cy="10" r="1.6" /></svg>
              </n-button>
            </n-dropdown>
          </header>

          <p v-if="key.description" class="desc">{{ key.description }}</p>

          <div class="usage">
            <span class="label">{{ t('keysPage.usedBy') }}</span>
            <span v-for="s in usedBy(key)" :key="s" class="chip">{{ s }}</span>
            <span v-if="key.system_default && instance.isSystem && unboundServers.length" class="chip soft">{{ t('keysPage.unboundServers', { n: unboundServers.length }) }}</span>
            <span v-if="!usedBy(key).length && !(key.system_default && instance.isSystem && unboundServers.length)" class="none">{{ t('keysPage.unused') }}</span>
          </div>

          <code v-if="key.public_key" class="pub" :title="key.public_key">{{ key.public_key }}</code>
          <p v-else class="none">{{ t('keys.noPublicKey') }}</p>

          <footer class="foot">
            <n-button size="small" secondary @click="openEdit(key)">{{ t('common.edit') }}</n-button>
            <n-button size="small" tertiary :disabled="!key.public_key" @click="copy(key.public_key ?? '', t('common.copied'))">{{ t('keys.copyPublic') }}</n-button>
            <n-button size="small" type="primary" ghost :disabled="!key.public_key" @click="copy(authorizeCommand(key), t('problem.commandCopied'))">{{ t('problem.copyCommand') }}</n-button>
          </footer>
        </article>
      </div>
    </div>

    <n-modal v-model:show="showModal" :title="editingName ? t('keys.editTitle') : t('keysPage.addTitle')" preset="dialog" style="width:600px;max-width:calc(100vw - 32px)">
      <n-form label-placement="top" style="margin-top:8px">
        <n-form-item :label="t('keys.fields.name')" :validation-status="submitted && errors.name ? 'error' : undefined" :feedback="submitted ? errors.name : ''">
          <n-input v-model:value="form.name" placeholder="deploy-key" />
        </n-form-item>
        <n-form-item :label="editingName ? t('keysPage.replaceMaterial') : t('keys.fields.privateKey')">
          <n-space vertical :size="8" style="width:100%">
            <div class="segmented">
              <button type="button" :class="{ active: source === 'paste' }" @click="source = 'paste'">{{ t('keysPage.paste') }}</button>
              <button type="button" :class="{ active: source === 'file' }" @click="source = 'file'">{{ t('keysPage.chooseFile') }}</button>
            </div>
            <n-input v-if="source === 'paste'" v-model:value="form.private_key" type="textarea" :rows="6" placeholder="-----BEGIN OPENSSH PRIVATE KEY-----" class="mono" />
            <label v-else class="file-pick">
              <input type="file" @change="onFileChange" />
              <span>{{ pickedFile || t('keysPage.pickHint') }}</span>
            </label>
            <n-text depth="3" style="font-size:12px">{{ t(editingName ? 'keys.replaceHint' : 'keys.uploadHint') }}</n-text>
          </n-space>
        </n-form-item>
        <n-form-item :label="t('keys.fields.description')">
          <n-input v-model:value="form.description" :placeholder="t('keysPage.descriptionPlaceholder')" />
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
.page-toolbar { display: flex; align-items: center; justify-content: space-between; padding: 10px 20px; background: var(--color-surface); border-bottom: 1px solid var(--color-border); flex-shrink: 0; }
.page-title { font-size: 14px; font-weight: 600; color: var(--color-text); }
.page-body { flex: 1; overflow: auto; padding: 20px; }
.intro { font-size: 12.5px; line-height: 1.6; color: var(--color-text-tertiary); max-width: 760px; margin-bottom: 16px; }

.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 12px; align-items: start; }
.key {
  display: flex; flex-direction: column; gap: 10px;
  padding: 14px 16px 12px; border-radius: 14px;
  background: var(--color-surface); border: 1px solid var(--color-border);
  box-shadow: 0 1px 2px var(--color-shadow-soft);
}
.key.primary { border-color: var(--color-accent-border); }
.head { display: flex; align-items: center; gap: 10px; min-width: 0; }
.icon {
  width: 34px; height: 34px; border-radius: 10px; flex-shrink: 0;
  display: inline-flex; align-items: center; justify-content: center;
  color: var(--color-accent); background: var(--color-accent-bg);
}
.icon svg { width: 18px; height: 18px; fill: none; stroke: currentColor; stroke-width: 1.6; stroke-linecap: round; }
.titles { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.name { font-size: 14.5px; font-weight: 650; color: var(--color-text); display: flex; align-items: center; gap: 8px; min-width: 0; }
.badge { font-size: 10.5px; font-weight: 600; padding: 1px 7px; border-radius: 999px; color: var(--color-accent-strong); background: var(--color-accent-bg); white-space: nowrap; }
.meta { font-family: var(--font-mono); font-size: 11.5px; color: var(--color-text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.more { width: 16px; height: 16px; fill: currentColor; }
.desc { font-size: 12.5px; color: var(--color-text-secondary); line-height: 1.5; }
.usage { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; font-size: 12px; }
.usage .label { color: var(--color-text-muted); margin-right: 2px; }
.chip { padding: 1px 8px; border-radius: 999px; background: var(--color-surface-alt); border: 1px solid var(--color-border); color: var(--color-text-secondary); }
.chip.soft { border-style: dashed; }
.none { font-size: 12px; color: var(--color-text-muted); }
.pub {
  font-family: var(--font-mono); font-size: 11px; color: var(--color-text-secondary);
  background: var(--color-surface-alt); border: 1px solid var(--color-border); border-radius: 8px;
  padding: 6px 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block;
}
.foot { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; border-top: 1px solid var(--color-divider-soft); padding-top: 10px; }

.segmented { display: inline-flex; border: 1px solid var(--color-border); border-radius: 8px; overflow: hidden; align-self: flex-start; }
.segmented button { font: inherit; font-size: 12px; padding: 5px 12px; cursor: pointer; border: 0; background: var(--color-surface); color: var(--color-text-secondary); }
.segmented button + button { border-left: 1px solid var(--color-border); }
.segmented button.active { background: var(--color-text); color: var(--color-surface); }
.mono :deep(textarea) { font-family: var(--font-mono); font-size: 12px; }
.file-pick {
  display: flex; align-items: center; justify-content: center; gap: 8px; cursor: pointer;
  padding: 22px 12px; border-radius: 10px; border: 1.5px dashed var(--color-border-strong);
  color: var(--color-text-tertiary); font-size: 13px; background: var(--color-surface-alt);
}
.file-pick input { display: none; }

.empty { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 10px; padding: 56px 16px; }
.empty h3 { font-size: 16px; font-weight: 650; color: var(--color-text); }
.empty p { font-size: 13px; color: var(--color-text-tertiary); max-width: 400px; line-height: 1.5; margin-bottom: 6px; }

@media (max-width: 640px) {
  .page-toolbar { padding: 10px 16px; }
  .page-body { padding: 16px; }
  .grid { grid-template-columns: 1fr; }
}
</style>
