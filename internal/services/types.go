// Package services is the shared abstraction layer called by CLI, HTTP API,
// and the tunnel manager. Keeping state here ensures CLI and Web UI cannot drift.
package services

import "github.com/HobaiRiku/ssh-tunnel-service/internal/config"

// TunnelState represents the runtime lifecycle state of a tunnel.
type TunnelState string

const (
StateStopped TunnelState = "stopped"
StateRunning TunnelState = "running"
StateError   TunnelState = "error"
)

// TunnelStatus pairs a config.Tunnel definition with live runtime information.
type TunnelStatus struct {
config.Tunnel
State TunnelState `json:"state"`
PID   int         `json:"pid,omitempty"`
Error string      `json:"error,omitempty"`
}
