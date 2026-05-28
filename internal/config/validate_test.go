package config

import "testing"

func TestApplyDefaultsSetsSSHDefaults(t *testing.T) {
	cfg := &Config{}

	ApplyDefaults(cfg, "/tmp/known_hosts")

	if cfg.App.SSHHostKeyPolicy != SSHHostKeyPolicyAcceptNew {
		t.Fatalf("expected default host key policy %q, got %q", SSHHostKeyPolicyAcceptNew, cfg.App.SSHHostKeyPolicy)
	}
	if cfg.App.SSHKnownHosts != "/tmp/known_hosts" {
		t.Fatalf("expected default known_hosts path to be applied, got %q", cfg.App.SSHKnownHosts)
	}
}

func TestValidateRejectsRelativeKnownHostsPath(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			SSHHostKeyPolicy: SSHHostKeyPolicyStrict,
			SSHKnownHosts:    "relative-known-hosts",
		},
	}

	if err := Validate(cfg); err == nil {
		t.Fatal("expected relative known_hosts path to be rejected")
	}
}
