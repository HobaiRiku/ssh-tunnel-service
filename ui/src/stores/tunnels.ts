import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, type TunnelStatus, type Tunnel } from '@/api/client'

export const useTunnelsStore = defineStore('tunnels', () => {
  const tunnels = ref<TunnelStatus[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function fetchTunnels() {
    loading.value = true
    error.value = null
    try {
      tunnels.value = await api.listTunnels()
    } catch (e: any) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function addTunnel(t: Tunnel) {
    await api.addTunnel(t)
    await fetchTunnels()
  }

  async function updateTunnel(id: string, t: Tunnel) {
    await api.updateTunnel(id, t)
    await fetchTunnels()
  }

  async function deleteTunnel(id: string) {
    await api.deleteTunnel(id)
    await fetchTunnels()
  }

  async function startTunnel(id: string) {
    await api.startTunnel(id)
    await fetchTunnels()
  }

  async function stopTunnel(id: string) {
    await api.stopTunnel(id)
    await fetchTunnels()
  }

  return {
    tunnels, loading, error,
    fetchTunnels, addTunnel, updateTunnel, deleteTunnel, startTunnel, stopTunnel
  }
})
