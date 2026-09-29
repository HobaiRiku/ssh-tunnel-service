<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import { useRouter } from 'vue-router'
import type { Remote, TunnelStatus } from '@/api/client'
import { copyText } from '@/clipboard'
import { useI18n } from '@/i18n'
import { useKeysStore } from '@/stores/keys'
import type { TunnelCardAction } from '@/tunnelText'

// Explains why a tunnel failed in plain words and offers the fix that matches
// the failure kind the service classified: authorize a key, correct the
// server, pick another port…
const props = defineProps<{
  tunnel: TunnelStatus
  remote?: Remote
}>()

const emit = defineEmits<{
  (e: 'action', action: TunnelCardAction): void
}>()

const { t } = useI18n()
const message = useMessage()
const router = useRouter()
const keyStore = useKeysStore()

type Fix = 'authorize' | 'bindKey' | 'editServer' | 'changePort' | 'none'

const kind = computed(() => props.tunnel.error_kind ?? '')
const params = computed(() => ({
  user: props.remote?.user ?? '',
  host: props.remote?.host ?? props.tunnel.remote,
  port: props.remote?.port ?? 22,
  bind: props.tunnel.bind_port,
  remote: props.tunnel.remote,
}))

// Map dispatch from failure kind to the words and the fix to offer.
const byKind: Record<string, { title: string; body: string; fix: Fix }> = {
  password_only: { title: 'problem.passwordOnlyTitle', body: 'problem.passwordOnlyBody', fix: 'authorize' },
  auth: { title: 'problem.authTitle', body: 'problem.authBody', fix: 'authorize' },
  host_key_unknown: { title: 'problem.hostKeyUnknownTitle', body: 'problem.hostKeyUnknownBody', fix: 'none' },
  host_key_changed: { title: 'problem.hostKeyChangedTitle', body: 'problem.hostKeyChangedBody', fix: 'none' },
  dns: { title: 'problem.dnsTitle', body: 'problem.dnsBody', fix: 'editServer' },
  refused: { title: 'problem.refusedTitle', body: 'problem.refusedBody', fix: 'editServer' },
  network: { title: 'problem.networkTitle', body: 'problem.networkBody', fix: 'editServer' },
  port_unavailable: { title: 'problem.portTitle', body: 'problem.portBody', fix: 'changePort' },
}

const view = computed<{ title: string; body: string; fix: Fix; details: string }>(() => {
  const entry = byKind[kind.value]
  if (!entry) return { title: t('problem.genericTitle'), body: props.tunnel.error, fix: 'none', details: '' }
  // An auth failure with no managed key means ssh used the machine's own
  // identities; the fix is to bind a managed key rather than authorize one.
  const fix: Fix = entry.fix === 'authorize' && !props.tunnel.error_key ? 'bindKey' : entry.fix
  // The raw OS error is only worth showing for a port problem (it says which
  // address and why); for the others the explanation above already covers it.
  const details = kind.value === 'port_unavailable' ? props.tunnel.error : ''
  return { title: t(entry.title, params.value), body: t(entry.body, params.value), fix, details }
})

const key = computed(() => keyStore.keys.find((k) => k.name === props.tunnel.error_key))
const authorizeCommand = computed(() => {
  const pub = key.value?.public_key?.trim()
  if (!pub) return ''
  return `mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '${pub}' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys`
})

onMounted(() => {
  if (props.tunnel.error_key && keyStore.keys.length === 0) void keyStore.fetchKeys()
})

async function copy(value: string, done: string) {
  if (!value) return
  const ok = await copyText(value)
  if (ok) message.success(done)
  else message.error(t('common.copyFailed'))
}

function editServer() {
  void router.push({ path: '/remotes', query: { edit: props.tunnel.remote } })
}
</script>

<template>
  <div class="problem" role="alert">
    <p class="title">{{ view.title }}</p>
    <p class="body">{{ view.body }}</p>

    <div v-if="view.fix === 'authorize'" class="key-box">
      <template v-if="key?.public_key">
        <p class="key-line">
          <span class="key-label">{{ t('problem.keyToAuthorize', { key: key.name, user: params.user }) }}</span>
        </p>
        <code class="key-pub" :title="key.public_key">{{ key.public_key }}</code>
      </template>
      <p v-else class="key-line muted">{{ t('problem.keyLoading', { key: tunnel.error_key ?? '' }) }}</p>
    </div>

    <p v-if="view.details && view.details !== view.body" class="details">{{ view.details }}</p>

    <div class="actions">
      <template v-if="view.fix === 'authorize'">
        <n-button size="tiny" type="primary" :disabled="!authorizeCommand" @click="copy(authorizeCommand, t('problem.commandCopied'))">{{ t('problem.copyCommand') }}</n-button>
        <n-button size="tiny" secondary :disabled="!key?.public_key" @click="copy(key?.public_key ?? '', t('common.copied'))">{{ t('keys.copyPublic') }}</n-button>
      </template>
      <n-button v-else-if="view.fix === 'bindKey' || view.fix === 'editServer'" size="tiny" type="primary" @click="editServer">{{ t('problem.editServer') }}</n-button>
      <n-button v-else-if="view.fix === 'changePort'" size="tiny" type="primary" @click="emit('action', 'edit')">{{ t('problem.changePort') }}</n-button>
      <n-button size="tiny" tertiary @click="emit('action', 'restart')">{{ t('problem.retry') }}</n-button>
    </div>
  </div>
</template>

<style scoped>
.problem {
  display: flex; flex-direction: column; gap: 6px;
  padding: 10px 12px; border-radius: 10px;
  background: var(--color-state-error-bg);
  border: 1px solid color-mix(in srgb, var(--color-danger) 25%, transparent);
}
.title { font-size: 13px; font-weight: 650; color: var(--color-danger); }
.body { font-size: 12px; line-height: 1.5; color: var(--color-text-secondary); }
.details { font-size: 11px; line-height: 1.45; color: var(--color-text-muted); }
.key-box { display: flex; flex-direction: column; gap: 4px; }
.key-line { font-size: 11.5px; color: var(--color-text-secondary); }
.key-line.muted { color: var(--color-text-muted); }
.key-pub {
  font-family: var(--font-mono); font-size: 11px; color: var(--color-text);
  background: var(--color-surface); border: 1px solid var(--color-border); border-radius: 6px;
  padding: 4px 6px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: block;
}
.actions { display: flex; gap: 6px; flex-wrap: wrap; margin-top: 2px; }
</style>
