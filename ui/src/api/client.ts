// API client for ssh-tunnel-service REST API

export interface SSHKey {
  id: string
  name: string
  file: string
  description: string
}

export interface SSHKeyPayload {
  id?: string
  name: string
  file_name?: string
  private_key?: string
  source_path?: string
  description: string
}

export interface Remote {
  id: string
  name: string
  host: string
  port: number
  user: string
  key_id: string
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

export interface TunnelCommandPreview {
  command: string
  args: string[]
}

interface BootstrapResponse {
  token?: string
}

let _token: string | null = null

export function getErrorMessage(error: unknown): string {
  if (error instanceof Error) return error.message
  return String(error)
}

export async function initAuth(): Promise<void> {
  const injected = window.__AUTH_TOKEN__
  if (injected) {
    _token = injected
    return
  }
  try {
    const res = await fetch('/api/bootstrap')
    if (res.ok) {
      const data = (await res.json()) as BootstrapResponse
      _token = data.token ?? null
    }
  } catch {
    _token = null
  }
}

const BASE = '/api'

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers = new Headers()
  if (body !== undefined) headers.set('Content-Type', 'application/json')
  if (_token) headers.set('Authorization', 'Bearer ' + _token)

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`${method} ${path}: ${res.status} ${text}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const api = {
  listKeys: () => req<SSHKey[]>('GET', '/keys'),
  addKey: (key: SSHKeyPayload) => req<SSHKey>('POST', '/keys', key),
  updateKey: (id: string, key: SSHKeyPayload) => req<SSHKey>('PUT', `/keys/${id}`, key),
  deleteKey: (id: string) => req<void>('DELETE', `/keys/${id}`),

  listRemotes: () => req<Remote[]>('GET', '/remotes'),
  addRemote: (remote: Omit<Remote, 'id'> & { id: string }) => req<Remote>('POST', '/remotes', remote),
  updateRemote: (id: string, remote: Remote) => req<Remote>('PUT', `/remotes/${id}`, remote),
  deleteRemote: (id: string) => req<void>('DELETE', `/remotes/${id}`),

  listTunnels: () => req<TunnelStatus[]>('GET', '/tunnels'),
  addTunnel: (tunnel: Tunnel) => req<Tunnel>('POST', '/tunnels', tunnel),
  updateTunnel: (id: string, tunnel: Tunnel) => req<Tunnel>('PUT', `/tunnels/${id}`, tunnel),
  deleteTunnel: (id: string) => req<void>('DELETE', `/tunnels/${id}`),
  getTunnelCommand: (id: string) => req<TunnelCommandPreview>('GET', `/tunnels/${id}/command`),
  startTunnel: (id: string) => req<void>('POST', `/tunnels/${id}/start`),
  stopTunnel: (id: string) => req<void>('POST', `/tunnels/${id}/stop`),

  health: () => req<{ ok: boolean }>('GET', '/health'),
}
