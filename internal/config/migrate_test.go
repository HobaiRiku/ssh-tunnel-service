package config

import "testing"

// TestMigrateLegacyConfigRewritesReferences verifies that a legacy id-keyed
// config is rewritten to be name-keyed, with key_id / remote_id references
// resolved to the corresponding names.
func TestMigrateLegacyConfigRewritesReferences(t *testing.T) {
	legacy := []byte(`
keys:
  - id: deploy-key
    name: "Primary deploy key"
    file: deploy-key
remotes:
  - id: bastion
    name: "Production Bastion"
    host: bastion.example.com
    port: 22
    user: deploy
    key_id: deploy-key
tunnels:
  - id: pg
    name: "Postgres"
    remote_id: bastion
    direction: "-L"
    bind_port: 15432
    target_host: db.internal
    target_port: 5432
`)

	cfg, err := parseConfig(legacy)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if got := cfg.Remotes[0].Key; got != "Primary deploy key" {
		t.Fatalf("remote key reference = %q, want key name", got)
	}
	if got := cfg.Tunnels[0].Remote; got != "Production Bastion" {
		t.Fatalf("tunnel remote reference = %q, want remote name", got)
	}
}

// TestMigrateFallsBackOnMissingAndDuplicateNames covers the "兜底" path:
// blank names fall back to the legacy id, and duplicate names are deduped so
// the result still validates.
func TestMigrateFallsBackOnMissingAndDuplicateNames(t *testing.T) {
	legacy := []byte(`
remotes:
  - id: alpha
    host: a.example.com
    port: 22
    user: u
  - id: alpha-2
    name: "alpha"
    host: b.example.com
    port: 22
    user: u
tunnels:
  - id: t1
    remote_id: alpha
    direction: "-L"
    bind_port: 1000
    target_host: x
    target_port: 2000
`)

	cfg, err := parseConfig(legacy)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	ApplyDefaults(cfg, "/tmp/known_hosts")
	if err := Validate(cfg); err != nil {
		t.Fatalf("migrated config failed validation: %v", err)
	}
	if cfg.Remotes[0].Name == cfg.Remotes[1].Name {
		t.Fatalf("expected duplicate names to be deduped, both are %q", cfg.Remotes[0].Name)
	}
	// First remote had no name, so it falls back to its id "alpha"; the tunnel
	// references it by remote_id and must resolve to that fallback name.
	if cfg.Tunnels[0].Remote != cfg.Remotes[0].Name {
		t.Fatalf("tunnel remote = %q, want %q", cfg.Tunnels[0].Remote, cfg.Remotes[0].Name)
	}
}

func TestValidateNameRejectsBadNames(t *testing.T) {
	bad := []string{"", " leading", "trailing ", "has/slash", "back\\slash"}
	for _, name := range bad {
		if err := ValidateName("tunnel", name); err == nil {
			t.Fatalf("expected ValidateName(%q) to fail", name)
		}
	}
	for _, name := range []string{"ok", "with space", "名称", "db-forward.1"} {
		if err := ValidateName("tunnel", name); err != nil {
			t.Fatalf("expected ValidateName(%q) to pass, got %v", name, err)
		}
	}
}
