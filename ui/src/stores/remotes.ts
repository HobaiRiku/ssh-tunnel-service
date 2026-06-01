import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, getErrorMessage, type Remote } from '@/api/client'

export const useRemotesStore = defineStore('remotes', () => {
  const remotes = ref<Remote[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchRemotes() {
    loading.value = true
    error.value = null
    try {
      remotes.value = await api.listRemotes()
    } catch (caughtError: unknown) {
      error.value = getErrorMessage(caughtError)
    } finally {
      loading.value = false
    }
  }

  async function addRemote(r: Remote) {
    await api.addRemote(r)
    await fetchRemotes()
  }

  async function updateRemote(name: string, r: Remote) {
    await api.updateRemote(name, r)
    await fetchRemotes()
  }

  async function deleteRemote(name: string) {
    await api.deleteRemote(name)
    await fetchRemotes()
  }

  return { remotes, loading, error, fetchRemotes, addRemote, updateRemote, deleteRemote }
})
