// Package services is the shared abstraction layer called by CLI, HTTP API,
// and the tunnel manager. Keeping state here ensures CLI and Web UI cannot drift.
package services

import (
	"errors"

	"ssh-tunnel-service/internal/config"
)

// ErrNotFound is returned by registry lookups when the requested resource does
// not exist. Callers should use errors.Is to check for this condition.
var ErrNotFound = errors.New("not found")

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

// TunnelCommandPreview is the shell command equivalent for a configured tunnel.
type TunnelCommandPreview struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// SSHKeyInput is the payload used to create or update a managed SSH key.
type SSHKeyInput struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	SourcePath  string `json:"source_path,omitempty"`
}
