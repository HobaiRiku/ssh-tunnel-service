// API client for ssh-tunnel-service REST API

export interface Remote {
  id: string
  name: string
  host: string
  port: number
  user: string
  description: string
}

export interface Tunnel {
  id: string
  name: string
  remote_id: string
  direction: '-L' | '-R'
  bind_address: string
  bind_port: number
  target_host: string
  target_port: number
  ssh_options: string[]
  auto_start: boolean
  description: string
}

export interface TunnelStatus extends Tunnel {
  state: 'stopped' | 'running' | 'error'
  pid: number
  error: string
}

export interface TopologyNode {
  id: string
  type: string
  data: {
    label: string
    direction?: string
    bindAddress?: string
    bindPort?: number
    state?: string
    host?: string
    port?: number
    user?: string
  }
  position: { x: number; y: number }
}

export interface TopologyEdge {
  id: string
  source: string
  target: string
  label: string
  animated?: boolean
}

export interface TopologyGraph {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

const BASE = '/api'

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${method} ${path}: ${res.status} ${text}`)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Remotes
  listRemotes: () => req<Remote[]>('GET', '/remotes'),
  addRemote: (r: Omit<Remote, 'id'> & { id: string }) => req<Remote>('POST', '/remotes', r),
  updateRemote: (id: string, r: Remote) => req<Remote>('PUT', `/remotes/${id}`, r),
  deleteRemote: (id: string) => req<void>('DELETE', `/remotes/${id}`),

  // Tunnels
  listTunnels: () => req<TunnelStatus[]>('GET', '/tunnels'),
  addTunnel: (t: Tunnel) => req<Tunnel>('POST', '/tunnels', t),
  updateTunnel: (id: string, t: Tunnel) => req<Tunnel>('PUT', `/tunnels/${id}`, t),
  deleteTunnel: (id: string) => req<void>('DELETE', `/tunnels/${id}`),
  startTunnel: (id: string) => req<void>('POST', `/tunnels/${id}/start`),
  stopTunnel: (id: string) => req<void>('POST', `/tunnels/${id}/stop`),

  // Topology
  topology: () => req<TopologyGraph>('GET', '/topology'),

  // Health
  health: () => req<{ status: string }>('GET', '/health')
}
