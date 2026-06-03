package config

import (
	"path/filepath"
	"testing"
)

// TestWriteExampleProducesValidConfig guards the bundled example against drift:
// the config written on first run must parse and pass validation. The example
// now starts empty by design — resources are added via the CLI/UI — so it must
// load cleanly with no keys/remotes/tunnels.
func TestWriteExampleProducesValidConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := WriteExample(path, 0o600); err != nil {
		t.Fatalf("WriteExample returned error: %v", err)
	}

	cfg, err := LoadWithDefaults(path, "/tmp/known_hosts")
	if err != nil {
		t.Fatalf("example config failed to load/validate: %v", err)
	}

	if len(cfg.Keys) != 0 || len(cfg.Remotes) != 0 || len(cfg.Tunnels) != 0 {
		t.Fatalf("expected an empty example config, got %d keys, %d remotes, %d tunnels",
			len(cfg.Keys), len(cfg.Remotes), len(cfg.Tunnels))
	}
	if cfg.App.HTTPListen == "" {
		t.Fatal("expected app.http_listen to be set in the example")
	}
}
