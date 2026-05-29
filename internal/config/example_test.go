package config

import (
	"path/filepath"
	"testing"
)

// TestWriteExampleProducesValidConfig guards the bundled example against drift:
// the config written on first run must parse and pass validation, otherwise a
// fresh install would fail to start.
func TestWriteExampleProducesValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := WriteExample(path, 0o600); err != nil {
		t.Fatalf("WriteExample returned error: %v", err)
	}

	cfg, err := LoadWithDefaults(path, "/tmp/known_hosts")
	if err != nil {
		t.Fatalf("example config failed to load/validate: %v", err)
	}

	if len(cfg.Remotes) == 0 {
		t.Fatal("expected example to define at least one remote")
	}
	if len(cfg.Tunnels) == 0 {
		t.Fatal("expected example to define at least one tunnel")
	}

	// Every tunnel must reference a remote that actually exists.
	remoteIDs := map[string]bool{}
	for _, r := range cfg.Remotes {
		remoteIDs[r.ID] = true
	}
	for _, tn := range cfg.Tunnels {
		if !remoteIDs[tn.RemoteID] {
			t.Fatalf("tunnel %q references unknown remote %q", tn.ID, tn.RemoteID)
		}
	}
}
