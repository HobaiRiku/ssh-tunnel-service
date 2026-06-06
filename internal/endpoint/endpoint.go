// Package endpoint implements runtime service discovery, decoupling the CLI from
// the service's data directory (home).
//
// A running instance advertises itself by writing a small endpoint file to a
// well-known, home-independent location. The CLI reads it to learn the API
// address, then fetches the token via the loopback-only /api/bootstrap endpoint
// — so it never needs read access to the service's home or token file.
//
// Two scopes mirror the two ways the service runs, and discovery prefers the
// user instance over the system one (matching the D-Bus session/system model):
//
//	ScopeUser    — `ssh-tunnel run` started by an unprivileged user
//	ScopeSystem  — the installed, elevated system service
//
// An explicit --home / SSH_TUNNEL_HOME always bypasses discovery and addresses
// that specific instance directly (see cmd/http.go).
package endpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ssh-tunnel-service/internal/elevate"
)

// Scope identifies which class of instance an endpoint belongs to.
type Scope string

const (
	ScopeUser   Scope = "user"
	ScopeSystem Scope = "system"

	fileName = "endpoint.json"
)

// Info is the advertised discovery record. Address is the only field the CLI
// strictly needs; the rest aid diagnostics (`status`, `tail`).
type Info struct {
	Address string `json:"address"`
	Home    string `json:"home"`
	PID     int    `json:"pid"`
	Scope   Scope  `json:"scope"`
}

// CurrentScope reports the scope the current process should advertise under,
// based on whether it is running elevated.
func CurrentScope() Scope {
	if elevate.IsElevated() {
		return ScopeSystem
	}
	return ScopeUser
}

// Write advertises addr/home under the current scope and returns the file path.
// Best-effort: callers treat a write error as non-fatal (discovery simply falls
// back to other mechanisms) but should log it.
func Write(addr, home string) (string, error) {
	scope := CurrentScope()
	dir, err := scopeDir(scope)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create runtime dir %s: %w", dir, err)
	}
	info := Info{Address: normalizeAddr(addr), Home: home, PID: os.Getpid(), Scope: scope}
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, fileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", fmt.Errorf("write endpoint %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("commit endpoint %s: %w", path, err)
	}
	return path, nil
}

// Remove deletes the current scope's endpoint file. Safe to call if absent.
func Remove() {
	if dir, err := scopeDir(CurrentScope()); err == nil {
		os.Remove(filepath.Join(dir, fileName))
	}
}

// Discover returns advertised instances in client-preference order: the user
// instance first, then the system one. Stale or unreadable files are skipped.
func Discover() []Info {
	var out []Info
	for _, scope := range []Scope{ScopeUser, ScopeSystem} {
		if info, ok := read(scope); ok {
			out = append(out, info)
		}
	}
	return out
}

// Lookup returns the advertised instance for a single scope, if present.
func Lookup(scope Scope) (Info, bool) {
	return read(scope)
}

func read(scope Scope) (Info, bool) {
	dir, err := scopeDir(scope)
	if err != nil {
		return Info{}, false
	}
	data, err := os.ReadFile(filepath.Join(dir, fileName))
	if err != nil {
		return Info{}, false
	}
	var info Info
	if json.Unmarshal(data, &info) != nil || info.Address == "" {
		return Info{}, false
	}
	info.Scope = scope
	return info, true
}

// normalizeAddr rewrites a wildcard bind address to loopback so the CLI (always
// a loopback client) can connect and pass the /api/bootstrap origin check.
func normalizeAddr(addr string) string {
	host, port, found := strings.Cut(addr, ":")
	if !found {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		return "127.0.0.1:" + port
	}
	return addr
}
