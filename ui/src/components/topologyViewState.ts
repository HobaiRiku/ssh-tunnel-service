// Module-scoped topology view state. Persists across remounts within a session
// (e.g. toggling the table/topology view, navigating away and back) so the
// canvas keeps the user's zoom, vertical scroll and selected remote tab.
export interface TopologyViewState {
  zoom?: number
  y?: number
  remoteId?: string | null
}

export const topologyViewState: TopologyViewState = {}
