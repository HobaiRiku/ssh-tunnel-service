package config

import (
	"path/filepath"
	"testing"
)

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

func TestValidateRejectsMissingKnownHostsPath(t *testing.T) {
	tests := []SSHHostKeyPolicy{
		SSHHostKeyPolicyAcceptNew,
		SSHHostKeyPolicyStrict,
	}

	for _, policy := range tests {
		t.Run(string(policy), func(t *testing.T) {
			cfg := &Config{
				App: AppConfig{
					SSHHostKeyPolicy: policy,
				},
			}

			if err := Validate(cfg); err == nil {
				t.Fatalf("expected missing known_hosts path to be rejected for policy %q", policy)
			}
		})
	}
}

func TestValidateRejectsUnknownRemoteKey(t *testing.T) {
	cfg := &Config{
		App:     AppConfig{SSHHostKeyPolicy: SSHHostKeyPolicyInsecure},
		Remotes: []Remote{{ID: "remote-a", Name: "A", Host: "host", Port: 22, User: "user", KeyID: "missing"}},
	}

	if err := Validate(cfg); err == nil {
		t.Fatal("expected unknown remote key to be rejected")
	}
}

func TestLoadWithDefaultsAppliesKnownHostsPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := WriteRaw(path, []byte("app: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadWithDefaults(path, "/tmp/known_hosts")
	if err != nil {
		t.Fatalf("LoadWithDefaults returned error: %v", err)
	}
	if cfg.App.SSHKnownHosts != "/tmp/known_hosts" {
		t.Fatalf("expected known_hosts default to be applied, got %q", cfg.App.SSHKnownHosts)
	}
}
