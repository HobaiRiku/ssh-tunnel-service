// API client for ssh-tunnel-service REST API.
// Every resource is identified by its unique `name`; there is no `id`.

export interface SSHKey {
  name: string
  file: string
  description: string
  public_key?: string
  // Set on the key designated as the system service's default identity.
  system_default?: boolean
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
  // Direct tunnels hand the port straight to ssh and are not metered.
  direct?: boolean
  description: string
}

// Live traffic of one tunnel (or the aggregate). "up" is traffic sent towards
// the forwarded service, "down" traffic coming back from it; rates are bytes/s.
export interface TrafficCounters {
  up_bytes: number
  down_bytes: number
  up_rate: number
  down_rate: number
  active_conns: number
  total_conns: number
}

export interface TrafficPoint {
  t: number // unix ms
  up: number
  down: number
  conns: number
}

export interface TrafficSnapshot {
  at: string
  interval_ms: number
  total: TrafficCounters
  tunnels: Record<string, TrafficCounters>
  history?: {
    total: TrafficPoint[]
    tunnels: Record<string, TrafficPoint[]>
  }
}

// Persisted long-term traffic (see /api/traffic/usage and /history).
export interface UsageTotals {
  up_bytes: number
  down_bytes: number
}

export interface TunnelUsage {
  today: UsageTotals
  month: UsageTotals
  all: UsageTotals
}

export interface UsageSummary {
  at: string
  total: TunnelUsage
  tunnels: Record<string, TunnelUsage>
}

export type UsageRange = '24h' | '30d' | '1y'

// Buckets are [start (unix seconds), up bytes, down bytes]; slots without
// traffic are omitted.
export interface UsageHistory {
  range: UsageRange
  step_seconds: number
  from: number
  to: number
  points: [number, number, number][]
}

export interface TunnelStatus extends Tunnel {
  state: 'stopped' | 'running' | 'error'
  pid: number
  error: string
  // Why the tunnel failed, when the service recognised the cause, and the
  // managed key it offered (for authentication failures).
  error_kind?: 'auth' | 'password_only' | 'host_key_unknown' | 'host_key_changed' | 'dns' | 'refused' | 'network' | 'port_unavailable'
  error_key?: string
  traffic?: TrafficCounters
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

// openTrafficStream connects to /api/traffic/stream. Browsers cannot set an
// Authorization header on a WebSocket, so the token travels as a
// "bearer.<token>" subprotocol; the server answers with "ssh-tunnel".
export function openTrafficStream(): WebSocket {
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const protocols = ['ssh-tunnel']
  if (_token) protocols.push('bearer.' + _token)
  return new WebSocket(`${scheme}://${window.location.host}${BASE}/traffic/stream`, protocols)
}

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
  setDefaultKey: (name: string) => req<void>('PUT', `/keys/${seg(name)}/default`),

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
  freePort: (from: number) => req<{ port: number }>('GET', `/ports/free?from=${from}`),
  trafficUsage: () => req<UsageSummary>('GET', '/traffic/usage'),
  trafficHistory: (range: UsageRange, tunnel?: string) =>
    req<UsageHistory>('GET', `/traffic/history?range=${range}${tunnel ? `&tunnel=${encodeURIComponent(tunnel)}` : ''}`),
  traffic: (history = false) => req<TrafficSnapshot>('GET', `/traffic${history ? '?history=true' : ''}`),

  health: () => req<{ ok: boolean }>('GET', '/health'),
}
