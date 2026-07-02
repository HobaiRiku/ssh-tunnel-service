<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { NButton, NModal, NSpace, NText, useMessage } from 'naive-ui'
import { api, type InstanceInfo } from '@/api/client'
import { copyText } from '@/clipboard'
import { useI18n } from '@/i18n'

const { t } = useI18n()
const message = useMessage()

// Instance identity: which instance this UI is talking to, mirroring the CLI
// banner/status so users always know (and can copy) what they are operating on.
const instance = ref<InstanceInfo | null>(null)
const showModal = ref(false)

const instanceLabel = computed(() => {
  const info = instance.value
  if (!info) return ''
  const tail = info.home ? info.home.split(/[\\/]/).filter(Boolean).pop() : ''
  const scope = info.scope || 'instance'
  return info.scope === 'custom' && tail ? `${scope} · ${tail}` : scope
})

// formatUptime mirrors cmd/status.go's formatUptime so the modal reads the same
// as `ssh-tunnel status`.
function formatUptime(seconds: number): string {
  let s = Math.max(0, Math.floor(seconds))
  const d = Math.floor(s / 86400); s %= 86400
  const h = Math.floor(s / 3600); s %= 3600
  const m = Math.floor(s / 60); s %= 60
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

interface DetailRow {
  key: string
  label: string
  value: string
}

const rows = computed<DetailRow[]>(() => {
  const info = instance.value
  if (!info) return []
  return [
    { key: 'status', label: t('instance.status'), value: t('common.running') },
    { key: 'scope', label: t('instance.scope'), value: info.scope || '—' },
    { key: 'address', label: t('instance.address'), value: info.address || '—' },
    { key: 'home', label: t('instance.home'), value: info.home || '—' },
    { key: 'pid', label: t('instance.pid'), value: String(info.pid) },
    { key: 'version', label: t('instance.version'), value: info.version || '—' },
    { key: 'uptime', label: t('instance.uptime'), value: formatUptime(info.uptime_seconds) },
  ]
})

// plainText renders the same key/value block a user would copy out of the
// `status` command, padded so it lines up when pasted into a terminal or note.
const plainText = computed(() => {
  const width = rows.value.reduce((max, r) => Math.max(max, r.label.length), 0)
  return rows.value.map((r) => `${(r.label + ':').padEnd(width + 3)}${r.value}`).join('\n')
})

async function copyValue(value: string) {
  const ok = await copyText(value)
  if (ok) message.success(t('common.copied'))
  else message.error(t('common.copyFailed'))
}

async function copyAll() {
  await copyValue(plainText.value)
}

onMounted(async () => {
  try {
    instance.value = await api.instance()
  } catch {
    instance.value = null
  }
})
</script>

<template>
  <button
    v-if="instanceLabel"
    type="button"
    class="instance-badge"
    :title="t('instance.viewHint')"
    @click="showModal = true"
  >{{ instanceLabel }}</button>

  <n-modal
    v-model:show="showModal"
    :title="t('instance.title')"
    preset="dialog"
    style="width:560px"
  >
    <div v-if="instance" class="instance-detail">
      <table class="detail-table">
        <tbody>
          <tr v-for="row in rows" :key="row.key">
            <th>{{ row.label }}</th>
            <td>
              <span class="detail-value">{{ row.value }}</span>
              <n-button
                text
                size="tiny"
                class="detail-copy"
                :title="t('common.copy')"
                @click="copyValue(row.value)"
              >{{ t('common.copy') }}</n-button>
            </td>
          </tr>
        </tbody>
      </table>
      <n-text depth="3" style="font-size:12px">{{ t('instance.copyHint') }}</n-text>
    </div>
    <template #action>
      <n-space justify="end">
        <n-button @click="showModal = false">{{ t('common.close') }}</n-button>
        <n-button type="primary" @click="copyAll">{{ t('instance.copyAll') }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.instance-badge {
  display: inline-flex;
  align-items: center;
  margin-left: auto;
  padding: 3px 10px;
  font-size: 12px;
  font-weight: 600;
  font-family: inherit;
  color: var(--color-accent);
  background: var(--color-accent-bg);
  border: 1px solid var(--color-accent-border);
  border-radius: 999px;
  white-space: nowrap;
  cursor: pointer;
  transition: background 0.15s, border-color 0.15s;
}

.instance-badge:hover {
  background: var(--color-surface-hover);
  border-color: var(--color-accent);
}

.instance-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 4px;
}

.detail-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.detail-table th {
  text-align: left;
  padding: 6px 12px 6px 0;
  color: var(--color-text-tertiary);
  font-weight: 500;
  white-space: nowrap;
  vertical-align: top;
  width: 1%;
}

.detail-table td {
  padding: 6px 0;
  color: var(--color-text);
}

.detail-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  word-break: break-all;
}

.detail-copy {
  margin-left: 10px;
  vertical-align: middle;
}
</style>
