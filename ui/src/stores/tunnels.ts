import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, getErrorMessage, type TunnelStatus, type Tunnel } from '@/api/client'

export const useTunnelsStore = defineStore('tunnels', () => {
  const tunnels = ref<TunnelStatus[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  // Background polling so live state (running/stopped/error) and reconnect
  // outcomes refresh on their own without the user clicking "Refresh".
  let pollTimer: ReturnType<typeof setInterval> | null = null
  let subscribers = 0

  async function fetchTunnels() {
    loading.value = true
    error.value = null
    try {
      tunnels.value = await api.listTunnels()
    } catch (caughtError: unknown) {
      error.value = getErrorMessage(caughtError)
    } finally {
      loading.value = false
    }
  }

  // Lightweight refresh used by the poller: no loading flag flicker, and it
  // stays silent on transient errors so the table doesn't flash an alert.
  async function refreshQuietly() {
    try {
      tunnels.value = await api.listTunnels()
      error.value = null
    } catch {
      /* keep last good data; the next tick may recover */
    }
  }

  function startAutoRefresh(intervalMs = 4000) {
    subscribers += 1
    if (pollTimer) return
    pollTimer = setInterval(() => {
      if (typeof document !== 'undefined' && document.hidden) return
      void refreshQuietly()
    }, intervalMs)
  }

  function stopAutoRefresh() {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0 && pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  async function addTunnel(t: Tunnel) {
    await api.addTunnel(t)
    await fetchTunnels()
  }

  async function updateTunnel(name: string, t: Tunnel) {
    await api.updateTunnel(name, t)
    await fetchTunnels()
  }

  async function deleteTunnel(name: string) {
    await api.deleteTunnel(name)
    await fetchTunnels()
  }

  async function startTunnel(name: string) {
    await api.startTunnel(name)
    await fetchTunnels()
  }

  async function stopTunnel(name: string) {
    await api.stopTunnel(name)
    await fetchTunnels()
  }

  return {
    tunnels, loading, error,
    fetchTunnels, addTunnel, updateTunnel, deleteTunnel, startTunnel, stopTunnel,
    startAutoRefresh, stopAutoRefresh,
  }
})
