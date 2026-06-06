package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"ssh-tunnel-service/internal/endpoint"
)

// contextScope enumerates how the CLI chooses which instance to attach to by
// default, persisted in the per-user context file.
const (
	scopeAuto   = "auto"
	scopeSystem = "system"
	scopeUser   = "user"
	scopeCustom = "custom"
)

// cliContext is the persisted CLI attach context. It records the active scope
// and, separately, the most recently used custom home so a bare `connect` can
// still offer it even after `connect --clear` resets the active scope to auto.
//
// It lives in the *invoking user's* home (~/.ssh-tunnel-service/context.json),
// never inside any instance's data root.
type cliContext struct {
	Scope      string `json:"scope"`
	Home       string `json:"home,omitempty"`
	LastCustom string `json:"last_custom,omitempty"`
}

// contextPath returns the per-user context file path. It deliberately uses the
// OS user home (not the resolved instance home) so the context is stable
// regardless of which instance is active.
func contextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return filepath.Join(home, ".ssh-tunnel-service", "context.json"), nil
}

// loadContext reads the persisted context, returning an auto context when none
// exists or it is unreadable.
func loadContext() cliContext {
	def := cliContext{Scope: scopeAuto}
	path, err := contextPath()
	if err != nil {
		return def
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return def
	}
	var ctx cliContext
	if json.Unmarshal(data, &ctx) != nil || ctx.Scope == "" {
		return def
	}
	return ctx
}

// saveContext persists the context atomically.
func saveContext(ctx cliContext) error {
	path, err := contextPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// scopeToEndpoint maps a context scope to its endpoint scope.
func scopeToEndpoint(scope string) (endpoint.Scope, bool) {
	switch scope {
	case scopeSystem:
		return endpoint.ScopeSystem, true
	case scopeUser:
		return endpoint.ScopeUser, true
	}
	return "", false
}
