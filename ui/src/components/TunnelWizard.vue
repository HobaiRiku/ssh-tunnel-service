<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { NButton, NInput, NInputNumber, NModal, NSelect, NSwitch, useMessage } from 'naive-ui'
import { api, getErrorMessage, type Remote, type Tunnel } from '@/api/client'
import { useI18n } from '@/i18n'
import { useTunnelsStore } from '@/stores/tunnels'
import { validateName } from '@/validation'

// Guided creation of a tunnel: pick what you want to do, pick the service
// (templates fill in the usual port), then confirm a sentence describing the
// result. Editing keeps using the full form in Tunnels.vue.
const props = defineProps<{
  show: boolean
  remotes: Remote[]
  initialRemote?: string
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'created', name: string): void
  (e: 'advanced'): void
}>()

const { t } = useI18n()
const message = useMessage()
const tunnelStore = useTunnelsStore()

type Direction = Tunnel['direction']
interface Template { id: string; label: string; port: number }

// Common services and their default ports. "custom" leaves the port to the user.
const templates: Template[] = [
  { id: 'postgres', label: 'PostgreSQL', port: 5432 },
  { id: 'mysql', label: 'MySQL', port: 3306 },
  { id: 'redis', label: 'Redis', port: 6379 },
  { id: 'mongo', label: 'MongoDB', port: 27017 },
  { id: 'http', label: 'HTTP', port: 80 },
  { id: 'https', label: 'HTTPS', port: 443 },
  { id: 'web', label: 'Web 3000', port: 3000 },
  { id: 'web8080', label: 'Web 8080', port: 8080 },
  { id: 'rdp', label: 'RDP', port: 3389 },
  { id: 'vnc', label: 'VNC', port: 5900 },
  { id: 'ssh', label: 'SSH', port: 22 },
  { id: 'custom', label: '', port: 0 },
]

const step = ref(1)
const saving = ref(false)
const suggesting = ref(false)
const submitted = ref(false)

interface WizardForm {
  direction: Direction
  template: string
  remote: string
  serviceHost: string
  servicePort: number | null
  listenPort: number | null
  exposure: 'private' | 'shared'
  name: string
  nameTouched: boolean
  description: string
  keepAlive: boolean
  meter: boolean
}

const form = reactive<WizardForm>({
  direction: '-L',
  template: 'postgres',
  remote: '',
  serviceHost: '127.0.0.1',
  servicePort: 5432,
  listenPort: 15432,
  exposure: 'private',
  name: '',
  nameTouched: false,
  description: '',
  keepAlive: true,
  meter: true,
})

const remoteOptions = computed(() => props.remotes.map((r) => ({ label: `${r.name} (${r.user}@${r.host})`, value: r.name })))
const template = computed(() => templates.find((x) => x.id === form.template))
const serviceLabel = computed(() => template.value && template.value.id !== 'custom' ? template.value.label : t('wizard.theService'))

function reset() {
  step.value = 1
  submitted.value = false
  Object.assign(form, {
    direction: '-L', template: 'postgres', remote: props.initialRemote || (props.remotes.length === 1 ? props.remotes[0].name : ''),
    serviceHost: '127.0.0.1', servicePort: 5432, listenPort: 15432, exposure: 'private',
    name: '', nameTouched: false, description: '', keepAlive: true, meter: true,
  })
}

watch(() => props.show, (open) => { if (open) reset() })

// Privileged ports need root to bind, so suggest a high port instead.
function preferredListenPort(port: number): number {
  return port < 1024 ? 10000 + port : port + 10000 <= 65535 ? port + 10000 : port
}

async function suggestListenPort() {
  const base = preferredListenPort(form.servicePort ?? 10000)
  if (form.direction === '-R') {
    form.listenPort = form.servicePort && form.servicePort >= 1024 ? form.servicePort : base
    return
  }
  suggesting.value = true
  try {
    form.listenPort = (await api.freePort(base)).port
  } catch {
    form.listenPort = base
  } finally {
    suggesting.value = false
  }
}

function pickTemplate(tpl: Template) {
  form.template = tpl.id
  if (tpl.port) form.servicePort = tpl.port
  void suggestListenPort()
}

function pickDirection(dir: Direction) {
  form.direction = dir
  if (dir === '-R' && form.template === 'postgres') {
    form.template = 'web'
    form.servicePort = 3000
  }
  void suggestListenPort()
  step.value = 2
}

const suggestedName = computed(() => {
  const base = template.value && template.value.id !== 'custom' ? template.value.id : `port-${form.servicePort ?? ''}`
  return [base, form.remote].filter(Boolean).join('-')
})
watch(suggestedName, (name) => { if (!form.nameTouched) form.name = name }, { immediate: true })

const bindAddress = computed(() => form.exposure === 'shared' ? '0.0.0.0' : '127.0.0.1')

const stepTwoError = computed(() => {
  if (!form.remote) return t('validation.remoteRequired')
  if (!form.serviceHost.trim()) return t('validation.hostRequired')
  if (!validPort(form.servicePort) || !validPort(form.listenPort)) return t('wizard.portInvalid')
  return ''
})
const nameError = computed(() => validateName(form.name, t) ?? '')

function validPort(p: number | null): p is number {
  return typeof p === 'number' && Number.isInteger(p) && p >= 1 && p <= 65535
}

// The sentence the user confirms, e.g. "Open 127.0.0.1:15432 on this machine
// to reach PostgreSQL (127.0.0.1:5432) through prod."
const preview = computed(() => {
  const vars = {
    listen: `${form.direction === '-L' ? '127.0.0.1' : bindAddress.value === '0.0.0.0' ? (props.remotes.find((r) => r.name === form.remote)?.host ?? form.remote) : '127.0.0.1'}:${form.listenPort ?? '?'}`,
    service: serviceLabel.value,
    target: `${form.serviceHost}:${form.servicePort ?? '?'}`,
    remote: form.remote || '…',
  }
  return form.direction === '-L' ? t('wizard.previewLocal', vars) : t('wizard.previewRemote', vars)
})

function next() {
  submitted.value = true
  if (step.value === 2 && stepTwoError.value) return
  submitted.value = false
  step.value += 1
}

async function create() {
  submitted.value = true
  if (nameError.value || stepTwoError.value) return
  const tunnel: Tunnel = {
    name: form.name.trim(),
    remote: form.remote,
    direction: form.direction,
    bind_address: bindAddress.value,
    bind_port: form.listenPort as number,
    target_host: form.serviceHost.trim(),
    target_port: form.servicePort as number,
    ssh_options: [],
    auto_start: form.keepAlive,
    direct: !form.meter,
    description: form.description.trim(),
  }
  saving.value = true
  try {
    await tunnelStore.addTunnel(tunnel)
    // auto_start tunnels start on creation; otherwise start it once now.
    if (!tunnel.auto_start) await tunnelStore.startTunnel(tunnel.name)
    message.success(t('wizard.created', { name: tunnel.name }))
    emit('created', tunnel.name)
    emit('update:show', false)
  } catch (error: unknown) {
    message.error(getErrorMessage(error))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="wizard"
    :title="t('wizard.title')"
    style="width:640px;max-width:calc(100vw - 32px)"
    :bordered="false"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <ol class="steps" aria-hidden="true">
      <li v-for="n in 3" :key="n" :class="{ done: step > n, current: step === n }">
        <span class="step-dot">{{ n }}</span><span class="step-label">{{ t(`wizard.step${n}`) }}</span>
      </li>
    </ol>

    <!-- Step 1: what do you want to do? -->
    <div v-if="step === 1" class="choices">
      <button type="button" class="choice local" @click="pickDirection('-L')">
        <svg viewBox="0 0 160 70" aria-hidden="true" class="choice-art">
          <rect x="4" y="18" width="42" height="34" rx="8" class="node me" /><text x="25" y="39" text-anchor="middle">{{ t('wizard.me') }}</text>
          <rect x="114" y="18" width="42" height="34" rx="8" class="node srv" /><text x="135" y="39" text-anchor="middle">{{ t('wizard.server') }}</text>
          <path d="M112 35H50" class="flow" /><path d="M58 29l-8 6 8 6" class="flow" />
        </svg>
        <span class="choice-title">{{ t('tunnels.direction.localTitle') }}</span>
        <span class="choice-sub">{{ t('wizard.localExample') }}</span>
      </button>
      <button type="button" class="choice remote" @click="pickDirection('-R')">
        <svg viewBox="0 0 160 70" aria-hidden="true" class="choice-art">
          <rect x="4" y="18" width="42" height="34" rx="8" class="node me" /><text x="25" y="39" text-anchor="middle">{{ t('wizard.me') }}</text>
          <rect x="114" y="18" width="42" height="34" rx="8" class="node srv" /><text x="135" y="39" text-anchor="middle">{{ t('wizard.server') }}</text>
          <path d="M48 35H110" class="flow" /><path d="M102 29l8 6-8 6" class="flow" />
        </svg>
        <span class="choice-title">{{ t('tunnels.direction.remoteTitle') }}</span>
        <span class="choice-sub">{{ t('wizard.remoteExample') }}</span>
      </button>
    </div>

    <!-- Step 2: which service, where. -->
    <div v-else-if="step === 2" class="fields">
      <div class="field">
        <span class="label">{{ t('wizard.whichService') }}</span>
        <div class="templates">
          <button
            v-for="tpl in templates"
            :key="tpl.id"
            type="button"
            class="tpl"
            :class="{ active: form.template === tpl.id }"
            @click="pickTemplate(tpl)"
          >
            <span>{{ tpl.id === 'custom' ? t('wizard.custom') : tpl.label }}</span>
            <small v-if="tpl.port">{{ tpl.port }}</small>
          </button>
        </div>
      </div>

      <div class="field">
        <span class="label">{{ t('wizard.throughServer') }}</span>
        <n-select v-model:value="form.remote" :options="remoteOptions" :placeholder="t('wizard.pickServer')" :status="submitted && !form.remote ? 'error' : undefined" />
      </div>

      <div class="field">
        <span class="label">{{ form.direction === '-L' ? t('wizard.serviceFromServer') : t('wizard.serviceHere') }}</span>
        <div class="hostport">
          <n-input v-model:value="form.serviceHost" placeholder="127.0.0.1" />
          <span class="colon">:</span>
          <n-input-number v-model:value="form.servicePort" :min="1" :max="65535" :show-button="false" placeholder="5432" @blur="suggestListenPort" />
        </div>
        <span class="hint">{{ form.direction === '-L' ? t('wizard.serviceFromServerHint') : t('wizard.serviceHereHint') }}</span>
      </div>

      <div class="field">
        <span class="label">{{ form.direction === '-L' ? t('wizard.listenHere') : t('wizard.listenOnServer') }}</span>
        <div class="listen">
          <n-input-number v-model:value="form.listenPort" :min="1" :max="65535" :show-button="false" class="port" />
          <n-button v-if="form.direction === '-L'" size="small" tertiary :loading="suggesting" @click="suggestListenPort">{{ t('wizard.suggestPort') }}</n-button>
        </div>
        <div class="segmented">
          <button type="button" :class="{ active: form.exposure === 'private' }" @click="form.exposure = 'private'">
            {{ form.direction === '-L' ? t('wizard.onlyMe') : t('wizard.onlyServer') }}
          </button>
          <button type="button" :class="{ active: form.exposure === 'shared' }" @click="form.exposure = 'shared'">
            {{ form.direction === '-L' ? t('wizard.lan') : t('wizard.everyone') }}
          </button>
        </div>
        <span v-if="form.direction === '-R' && form.exposure === 'shared'" class="hint warn">{{ t('wizard.gatewayPortsHint') }}</span>
      </div>
      <p v-if="submitted && stepTwoError" class="error">{{ stepTwoError }}</p>
    </div>

    <!-- Step 3: confirm. -->
    <div v-else class="fields">
      <div class="preview" :class="form.direction === '-L' ? 'local' : 'remote'">{{ preview }}</div>
      <div class="field">
        <span class="label">{{ t('tunnels.fields.name') }}</span>
        <n-input v-model:value="form.name" :status="submitted && nameError ? 'error' : undefined" @update:value="form.nameTouched = true" />
        <span v-if="submitted && nameError" class="error">{{ nameError }}</span>
      </div>
      <div class="field">
        <span class="label">{{ t('tunnels.fields.description') }} <small>{{ t('common.optional') }}</small></span>
        <n-input v-model:value="form.description" :placeholder="t('wizard.descriptionPlaceholder')" />
      </div>
      <label class="toggle">
        <span><b>{{ t('wizard.keepAlive') }}</b><small>{{ t('tunnels.autoStartHint') }}</small></span>
        <n-switch v-model:value="form.keepAlive" />
      </label>
      <label class="toggle">
        <span><b>{{ t('tunnels.fields.meterTraffic') }}</b><small>{{ form.meter ? t('tunnels.meterHint') : t('tunnels.directHint') }}</small></span>
        <n-switch v-model:value="form.meter" />
      </label>
    </div>

    <template #footer>
      <div class="footer">
        <n-button quaternary size="small" @click="emit('advanced')">{{ t('wizard.advanced') }}</n-button>
        <div class="footer-right">
          <n-button v-if="step > 1" @click="step -= 1">{{ t('wizard.back') }}</n-button>
          <n-button v-if="step === 2" type="primary" @click="next">{{ t('wizard.next') }}</n-button>
          <n-button v-if="step === 3" type="primary" :loading="saving" @click="create">{{ t('wizard.create') }}</n-button>
        </div>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.steps { display: flex; gap: 8px; list-style: none; margin-bottom: 18px; }
.steps li { display: flex; align-items: center; gap: 6px; flex: 1; font-size: 12px; color: var(--color-text-muted); }
.steps li + li::before { content: ''; flex: 0 0 16px; height: 1px; background: var(--color-border); margin-right: 2px; }
.step-dot {
  width: 20px; height: 20px; border-radius: 50%; display: inline-flex; align-items: center; justify-content: center;
  font-size: 11px; font-weight: 700; border: 1.5px solid var(--color-border-strong); flex-shrink: 0;
}
.steps li.current { color: var(--color-text); font-weight: 600; }
.steps li.current .step-dot { border-color: var(--color-accent); color: var(--color-accent); }
.steps li.done .step-dot { background: var(--color-accent); border-color: var(--color-accent); color: #fff; }

.choices { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.choice {
  font: inherit; text-align: left; cursor: pointer;
  display: flex; flex-direction: column; gap: 6px; padding: 16px;
  border-radius: 14px; border: 1.5px solid var(--color-border); background: var(--color-surface);
  transition: border-color 0.15s, transform 0.15s, box-shadow 0.15s;
  --tone: var(--color-dir-local);
}
.choice.remote { --tone: var(--color-dir-remote); }
.choice:hover { border-color: var(--tone); box-shadow: 0 6px 18px var(--color-shadow-soft); transform: translateY(-1px); }
.choice-art { width: 100%; height: 70px; margin-bottom: 6px; }
.choice-art .node { fill: var(--color-surface-alt); stroke: var(--color-border-strong); stroke-width: 1.2; }
.choice-art text { font-size: 10px; fill: var(--color-text-secondary); }
.choice-art .flow { fill: none; stroke: var(--tone); stroke-width: 2.4; stroke-linecap: round; stroke-linejoin: round; }
.choice-title { font-size: 14.5px; font-weight: 650; color: var(--color-text); }
.choice-sub { font-size: 12px; line-height: 1.5; color: var(--color-text-tertiary); }

.fields { display: flex; flex-direction: column; gap: 16px; }
.field { display: flex; flex-direction: column; gap: 6px; }
.label { font-size: 12.5px; font-weight: 600; color: var(--color-text-secondary); }
.label small { font-weight: 400; color: var(--color-text-muted); margin-left: 4px; }
.hint { font-size: 11.5px; color: var(--color-text-muted); line-height: 1.45; }
.hint.warn { color: #b45309; }
.error { font-size: 12px; color: var(--color-danger); }
.templates { display: flex; flex-wrap: wrap; gap: 6px; }
.tpl {
  font: inherit; font-size: 12.5px; cursor: pointer;
  display: inline-flex; align-items: baseline; gap: 5px;
  padding: 5px 10px; border-radius: 8px; border: 1px solid var(--color-border);
  background: var(--color-surface); color: var(--color-text-secondary);
}
.tpl small { font-size: 10.5px; color: var(--color-text-muted); font-variant-numeric: tabular-nums; }
.tpl.active { border-color: var(--color-accent); background: var(--color-accent-bg); color: var(--color-accent-strong); }
.hostport { display: grid; grid-template-columns: 1fr auto 110px; align-items: center; gap: 6px; }
.colon { color: var(--color-text-muted); }
.listen { display: flex; gap: 8px; align-items: center; }
.listen .port { width: 140px; }
.segmented { display: inline-flex; border: 1px solid var(--color-border); border-radius: 8px; overflow: hidden; align-self: flex-start; }
.segmented button {
  font: inherit; font-size: 12px; padding: 5px 12px; cursor: pointer; border: 0;
  background: var(--color-surface); color: var(--color-text-secondary);
}
.segmented button + button { border-left: 1px solid var(--color-border); }
.segmented button.active { background: var(--color-text); color: var(--color-surface); }

.preview {
  font-size: 14px; line-height: 1.55; color: var(--color-text);
  padding: 12px 14px; border-radius: 12px; background: var(--color-surface-alt);
  border-left: 3px solid var(--color-dir-local);
}
.preview.remote { border-left-color: var(--color-dir-remote); }
.toggle { display: flex; align-items: center; justify-content: space-between; gap: 16px; cursor: pointer; }
.toggle span { display: flex; flex-direction: column; gap: 2px; }
.toggle b { font-size: 13px; font-weight: 600; color: var(--color-text); }
.toggle small { font-size: 11.5px; color: var(--color-text-muted); line-height: 1.4; }

.footer { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.footer-right { display: flex; gap: 8px; }

@media (max-width: 640px) {
  .choices { grid-template-columns: 1fr; }
  .step-label { display: none; }
}
</style>
