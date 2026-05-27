import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type Remote } from '@/api/client'

export const useRemotesStore = defineStore('remotes', () => {
  const remotes = ref<Remote[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchRemotes() {
    loading.value = true
    error.value = null
    try {
      remotes.value = await api.listRemotes()
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function addRemote(r: Remote) {
    await api.addRemote(r)
    await fetchRemotes()
  }

  async function updateRemote(id: string, r: Remote) {
    await api.updateRemote(id, r)
    await fetchRemotes()
  }

  async function deleteRemote(id: string) {
    await api.deleteRemote(id)
    await fetchRemotes()
  }

  return { remotes, loading, error, fetchRemotes, addRemote, updateRemote, deleteRemote }
})
