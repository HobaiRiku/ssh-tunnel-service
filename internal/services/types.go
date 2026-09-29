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

// ErrNotRunning is returned by Manager.Stop when the tunnel has no live process.
// Restart treats it as a benign "nothing to stop" rather than a failure.
var ErrNotRunning = errors.New("not running")

// ErrAlreadyExists is returned by registry Add operations when a resource with
// the given name already exists. Callers should use errors.Is to check for it.
var ErrAlreadyExists = errors.New("already exists")

// TunnelState represents the runtime lifecycle state of a tunnel.
type TunnelState string

const (
	StateStopped TunnelState = "stopped"
	StateRunning TunnelState = "running"
	StateError   TunnelState = "error"
)

// FailureKind classifies why a tunnel is in the error state, so clients can
// offer a matching fix (authorize a key, edit the server, pick another port…).
// An empty kind means the cause was not recognised; the message still says
// what happened.
type FailureKind string

const (
	FailureAuth            FailureKind = "auth"
	FailurePasswordOnly    FailureKind = "password_only"
	FailureHostKeyUnknown  FailureKind = "host_key_unknown"
	FailureHostKeyChanged  FailureKind = "host_key_changed"
	FailureDNS             FailureKind = "dns"
	FailureRefused         FailureKind = "refused"
	FailureNetwork         FailureKind = "network"
	FailurePortUnavailable FailureKind = "port_unavailable"
)

// Failure describes a tunnel's last failure.
type Failure struct {
	Kind    FailureKind
	Message string
	// Key is the managed key that was offered, when one was.
	Key string
}

// TunnelStatus pairs a config.Tunnel definition with live runtime information.
type TunnelStatus struct {
	config.Tunnel
	State TunnelState `json:"state"`
	PID   int         `json:"pid,omitempty"`
	Error string      `json:"error,omitempty"`
	// ErrorKind classifies Error (see FailureKind); ErrorKey names the managed
	// key the service authenticated with, for auth failures.
	ErrorKind FailureKind `json:"error_kind,omitempty"`
	ErrorKey  string      `json:"error_key,omitempty"`
	// Traffic is the tunnel's live traffic, or nil when it has never been
	// metered (a direct tunnel, or one that has not been started yet).
	Traffic *TrafficCounters `json:"traffic,omitempty"`
}

// TunnelCommandPreview is the shell command equivalent for a configured tunnel.
type TunnelCommandPreview struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

// SSHKeyInput is the payload used to create or update a managed SSH key.
// Keys are identified by Name; there is no separate id.
type SSHKeyInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	SourcePath  string `json:"-"`
}
