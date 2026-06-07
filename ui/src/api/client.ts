// API client for ssh-tunnel-service REST API.
// Every resource is identified by its unique `name`; there is no `id`.

export interface SSHKey {
  name: string
  file: string
  description: string
  public_key?: string
}

export interface SSHKeyPayload {
  name: string
  file_name?: string
  private_key?: string
  source_path?: string
  description: string
}

export interface Remote {
  name: string
  host: string
  port: number
  user: string
  key: string
  description: string
}

export interface Tunnel {
  name: string
  remote: string
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

export interface InstanceInfo {
  scope: string
  home: string
  address: string
  pid: number
  version: string
  uptime_seconds: number
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

// Names are used as path segments, so they must be URL-encoded.
const seg = (name: string) => encodeURIComponent(name)

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
    throw new Error(extractError(text) || `${method} ${path}: ${res.status}`)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

// The API returns errors as {"error": "..."}; surface that message directly so
// the UI shows a clean reason instead of a raw HTTP envelope.
function extractError(text: string): string {
  try {
    const parsed = JSON.parse(text) as { error?: string }
    if (parsed && typeof parsed.error === 'string') return parsed.error
  } catch {
    /* not JSON */
  }
  return text
}

export const api = {
  listKeys: () => req<SSHKey[]>('GET', '/keys'),
  addKey: (key: SSHKeyPayload) => req<SSHKey>('POST', '/keys', key),
  updateKey: (name: string, key: SSHKeyPayload) => req<SSHKey>('PUT', `/keys/${seg(name)}`, key),
  deleteKey: (name: string) => req<void>('DELETE', `/keys/${seg(name)}`),

  listRemotes: () => req<Remote[]>('GET', '/remotes'),
  addRemote: (remote: Remote) => req<Remote>('POST', '/remotes', remote),
  updateRemote: (name: string, remote: Remote) => req<Remote>('PUT', `/remotes/${seg(name)}`, remote),
  deleteRemote: (name: string) => req<void>('DELETE', `/remotes/${seg(name)}`),

  listTunnels: () => req<TunnelStatus[]>('GET', '/tunnels'),
  addTunnel: (tunnel: Tunnel) => req<Tunnel>('POST', '/tunnels', tunnel),
  updateTunnel: (name: string, tunnel: Tunnel) => req<Tunnel>('PUT', `/tunnels/${seg(name)}`, tunnel),
  deleteTunnel: (name: string) => req<void>('DELETE', `/tunnels/${seg(name)}`),
  getTunnelCommand: (name: string) => req<TunnelCommandPreview>('GET', `/tunnels/${seg(name)}/command`),
  startTunnel: (name: string) => req<void>('POST', `/tunnels/${seg(name)}/start`),
  stopTunnel: (name: string) => req<void>('POST', `/tunnels/${seg(name)}/stop`),
  restartTunnel: (name: string) => req<void>('POST', `/tunnels/${seg(name)}/restart`),

  instance: () => req<InstanceInfo>('GET', '/instance'),

  health: () => req<{ ok: boolean }>('GET', '/health'),
}
