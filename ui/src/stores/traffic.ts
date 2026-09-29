import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import { api, openTrafficStream, type TrafficCounters, type TrafficPoint, type TrafficSnapshot, type UsageSummary } from '@/api/client'

// How many one-second samples to keep client-side (matches the server's ring).
const HISTORY_LEN = 300

const EMPTY: TrafficCounters = { up_bytes: 0, down_bytes: 0, up_rate: 0, down_rate: 0, active_conns: 0, total_conns: 0 }

// Live traffic, fed by the /api/traffic/stream WebSocket: the first frame
// carries five minutes of history, every later frame one new sample which is
// appended here. The connection is shared app-wide (header + tunnel views) and
// reconnects with backoff when the service restarts.
export const useTrafficStore = defineStore('traffic', () => {
  const total = ref<TrafficCounters>({ ...EMPTY })
  const tunnels = shallowRef<Record<string, TrafficCounters>>({})
  const totalHistory = shallowRef<TrafficPoint[]>([])
  const history = shallowRef<Record<string, TrafficPoint[]>>({})
  const connected = ref(false)
  // Today / this month / all-time per tunnel, from the service's persisted
  // book. It changes slowly, so it is polled rather than streamed.
  const usage = shallowRef<UsageSummary | null>(null)
  let usageTimer: ReturnType<typeof setInterval> | null = null

  async function refreshUsage() {
    try {
      usage.value = await api.trafficUsage()
    } catch {
      /* keep the last figures; the next poll may succeed */
    }
  }

  let socket: WebSocket | null = null
  let retryTimer: ReturnType<typeof setTimeout> | null = null
  let retryDelay = 1000
  let wanted = false

  function apply(snap: TrafficSnapshot) {
    total.value = snap.total
    tunnels.value = snap.tunnels ?? {}
    if (snap.history) {
      totalHistory.value = snap.history.total ?? []
      history.value = snap.history.tunnels ?? {}
      return
    }
    const t = Date.parse(snap.at)
    totalHistory.value = append(totalHistory.value, point(t, snap.total))
    const next: Record<string, TrafficPoint[]> = {}
    for (const [name, c] of Object.entries(snap.tunnels ?? {})) {
      next[name] = append(history.value[name] ?? [], point(t, c))
    }
    history.value = next
  }

  function connect() {
    wanted = true
    if (!usageTimer) {
      void refreshUsage()
      usageTimer = setInterval(() => { void refreshUsage() }, 10_000)
    }
    if (socket) return
    const ws = openTrafficStream()
    socket = ws
    ws.onopen = () => {
      connected.value = true
      retryDelay = 1000
    }
    ws.onmessage = (event: MessageEvent<string>) => {
      try {
        apply(JSON.parse(event.data) as TrafficSnapshot)
      } catch {
        /* ignore a malformed frame; the next one replaces it */
      }
    }
    ws.onclose = () => {
      connected.value = false
      socket = null
      if (!wanted) return
      retryTimer = setTimeout(connect, retryDelay)
      retryDelay = Math.min(retryDelay * 2, 10_000)
    }
  }

  function disconnect() {
    wanted = false
    if (usageTimer) clearInterval(usageTimer)
    usageTimer = null
    if (retryTimer) clearTimeout(retryTimer)
    retryTimer = null
    socket?.close()
    socket = null
  }

  function forTunnel(name: string): TrafficCounters | undefined {
    return tunnels.value[name]
  }

  return { total, tunnels, totalHistory, history, connected, usage, connect, disconnect, forTunnel, refreshUsage }
})

function point(t: number, c: TrafficCounters): TrafficPoint {
  return { t, up: c.up_rate, down: c.down_rate, conns: c.active_conns }
}

function append(list: TrafficPoint[], p: TrafficPoint): TrafficPoint[] {
  const next = list.length >= HISTORY_LEN ? list.slice(list.length - HISTORY_LEN + 1) : list.slice()
  next.push(p)
  return next
}
