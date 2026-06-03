package services

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"ssh-tunnel-service/internal/config"
	"ssh-tunnel-service/internal/paths"
)

func newEmptyRegistry(t *testing.T) (*Registry, paths.Paths) {
	t.Helper()
	p := paths.Paths{Home: t.TempDir()}
	if err := p.EnsureTree(); err != nil {
		t.Fatalf("EnsureTree: %v", err)
	}
	cfg := &config.Config{App: config.AppConfig{
		HTTPListen:       config.DefaultHTTPListen,
		SSHHostKeyPolicy: config.SSHHostKeyPolicyAcceptNew,
		SSHKnownHosts:    p.KnownHosts(),
	}}
	return New(cfg, p, NewRuntime()), p
}

func TestEnsureSystemDefaultKeyGeneratesAndProtects(t *testing.T) {
	reg, _ := newEmptyRegistry(t)

	name, err := reg.EnsureSystemDefaultKey()
	if err != nil {
		t.Fatalf("EnsureSystemDefaultKey: %v", err)
	}
	if name != systemDefaultKeyName {
		t.Fatalf("default key name = %q, want %q", name, systemDefaultKeyName)
	}
	if got := reg.AppConfig().SystemDefaultKey; got != name {
		t.Fatalf("app.system_default_key = %q, want %q", got, name)
	}
	if _, err := reg.GetKey(name); err != nil {
		t.Fatalf("generated key not found: %v", err)
	}

	// Idempotent: a second call must not create a different key.
	again, err := reg.EnsureSystemDefaultKey()
	if err != nil || again != name {
		t.Fatalf("EnsureSystemDefaultKey not idempotent: got %q, %v", again, err)
	}
	if n := len(reg.ListKeys()); n != 1 {
		t.Fatalf("expected exactly 1 key after idempotent ensure, got %d", n)
	}

	// The designated default cannot be deleted until switched away.
	if err := reg.DeleteKey(name); err == nil {
		t.Fatal("expected deleting the system default key to be rejected")
	}
}

func TestManagerInjectsSystemDefaultKeyForUnboundRemote(t *testing.T) {
	reg, p := newEmptyRegistry(t)
	if _, err := reg.EnsureSystemDefaultKey(); err != nil {
		t.Fatalf("EnsureSystemDefaultKey: %v", err)
	}
	// Remote with no key bound; tunnel referencing it.
	if err := reg.AddRemote(config.Remote{Name: "r", Host: "h", Port: 22, User: "u"}); err != nil {
		t.Fatalf("AddRemote: %v", err)
	}
	if err := reg.AddTunnel(config.Tunnel{Name: "t", Remote: "r", Direction: config.DirectionLocal, BindAddress: "127.0.0.1", BindPort: 1, TargetHost: "x", TargetPort: 2}); err != nil {
		t.Fatalf("AddTunnel: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// System service: unbound remote gets the default key injected.
	sys := NewManager(context.Background(), reg, NewRuntime(), logger, true)
	_, _, key, _, err := sys.lookupTunnel("t")
	if err != nil {
		t.Fatalf("lookupTunnel(system): %v", err)
	}
	if key == nil || key.Name != systemDefaultKeyName {
		t.Fatalf("system service: expected default key injected, got %+v", key)
	}
	if !strings.Contains(sys.mustArgs(t, "t"), p.Keys()) {
		t.Fatalf("system service: expected -i under %s in args", p.Keys())
	}

	// Session run: unbound remote gets no key.
	sess := NewManager(context.Background(), reg, NewRuntime(), logger, false)
	_, _, key, _, err = sess.lookupTunnel("t")
	if err != nil {
		t.Fatalf("lookupTunnel(session): %v", err)
	}
	if key != nil {
		t.Fatalf("session run: expected no key, got %+v", key)
	}
}

// mustArgs builds the real ssh args (key included) for assertion convenience.
func (m *Manager) mustArgs(t *testing.T, name string) string {
	t.Helper()
	ts, remote, key, appCfg, err := m.lookupTunnel(name)
	if err != nil {
		t.Fatalf("lookupTunnel: %v", err)
	}
	return strings.Join(sshArgs(ts.Tunnel, remote, key, appCfg, m.reg), " ")
}
