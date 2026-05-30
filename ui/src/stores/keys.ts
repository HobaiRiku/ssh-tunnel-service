import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, getErrorMessage, type SSHKey, type SSHKeyPayload } from '@/api/client'

export const useKeysStore = defineStore('keys', () => {
  const keys = ref<SSHKey[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchKeys() {
    loading.value = true
    error.value = null
    try {
      keys.value = await api.listKeys()
    } catch (caughtError: unknown) {
      error.value = getErrorMessage(caughtError)
    } finally {
      loading.value = false
    }
  }

  async function addKey(key: SSHKeyPayload) {
    await api.addKey(key)
    await fetchKeys()
  }

  async function updateKey(id: string, key: SSHKeyPayload) {
    await api.updateKey(id, key)
    await fetchKeys()
  }

  async function deleteKey(id: string) {
    await api.deleteKey(id)
    await fetchKeys()
  }

  return { keys, loading, error, fetchKeys, addKey, updateKey, deleteKey }
})
