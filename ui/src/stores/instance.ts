import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api, type InstanceInfo } from '@/api/client'

// Which instance this UI talks to. Fetched once and shared: the header badge
// shows it, and the servers/keys pages need the scope to explain what an
// unbound server logs in with (system: the managed default key; user: the
// machine's own ssh identities).
export const useInstanceStore = defineStore('instance', () => {
  const info = ref<InstanceInfo | null>(null)
  let pending: Promise<void> | null = null

  function load(): Promise<void> {
    pending ??= (async () => {
      try {
        info.value = await api.instance()
      } catch {
        info.value = null
        pending = null
      }
    })()
    return pending
  }

  const isSystem = computed(() => info.value?.scope === 'system')

  return { info, isSystem, load }
})
