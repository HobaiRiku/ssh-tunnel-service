// Plain-language descriptions of a tunnel, shared by the overview cards and
// dialogs. Every tunnel is described the same way regardless of direction:
// an *entry* (where you connect) leading to a *service* (what you reach).
import type { Remote, Tunnel } from '@/api/client'
import { useI18n } from '@/i18n'

// Secondary actions offered from a tunnel card's menu.
export type TunnelCardAction = 'details' | 'restart' | 'edit' | 'command' | 'copyName' | 'delete'

const WILDCARD = new Set(['', '*', '0.0.0.0', '::', '[::]'])
const LOOPBACK = new Set(['127.0.0.1', 'localhost', '::1', '[::1]'])

export interface TunnelEnds {
  entryWhere: string
  entry: string
  serviceWhere: string
  service: string
}

// connectAddress is what a user types to use the tunnel: for -L a port on this
// machine, for -R a port on the remote server (its hostname when the listener
// is on every interface).
export function connectAddress(tunnel: Tunnel, remote?: Remote): string {
  const port = tunnel.bind_port
  const bind = tunnel.bind_address
  if (tunnel.direction === '-L') {
    return WILDCARD.has(bind) ? `127.0.0.1:${port}` : `${bind}:${port}`
  }
  if (WILDCARD.has(bind) && remote) return `${remote.host}:${port}`
  if (LOOPBACK.has(bind) || WILDCARD.has(bind)) return `127.0.0.1:${port}`
  return `${bind}:${port}`
}

export function useTunnelText() {
  const { t } = useI18n()

  function kind(tunnel: Tunnel): string {
    return tunnel.direction === '-L' ? t('tunnels.direction.localTitle') : t('tunnels.direction.remoteTitle')
  }

  function ends(tunnel: Tunnel): TunnelEnds {
    const bind = `${tunnel.bind_address}:${tunnel.bind_port}`
    const target = `${tunnel.target_host}:${tunnel.target_port}`
    if (tunnel.direction === '-L') {
      return {
        entryWhere: t('overview.onThisMachine'),
        entry: bind,
        serviceWhere: t('overview.seenFrom', { remote: tunnel.remote }),
        service: target,
      }
    }
    return {
      entryWhere: t('overview.onRemote', { remote: tunnel.remote }),
      entry: bind,
      serviceWhere: t('overview.onThisMachine'),
      service: target,
    }
  }

  // One sentence for tooltips and confirmations, e.g.
  // "127.0.0.1:15432 on this machine → db.internal:5432 as seen from prod".
  function sentence(tunnel: Tunnel): string {
    const e = ends(tunnel)
    return `${e.entry} (${e.entryWhere}) → ${e.service} (${e.serviceWhere})`
  }

  return { kind, ends, sentence }
}
