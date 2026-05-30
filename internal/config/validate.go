package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// GenerateToken returns a 32-byte cryptographically random hex string.
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// Validate checks the parsed config for required fields and consistency.
func Validate(cfg *Config) error {
	if cfg.App.SSHHostKeyPolicy != "" &&
		cfg.App.SSHHostKeyPolicy != SSHHostKeyPolicyAcceptNew &&
		cfg.App.SSHHostKeyPolicy != SSHHostKeyPolicyStrict &&
		cfg.App.SSHHostKeyPolicy != SSHHostKeyPolicyInsecure {
		return fmt.Errorf("app.ssh_host_key_policy must be one of %q, %q, %q",
			SSHHostKeyPolicyAcceptNew, SSHHostKeyPolicyStrict, SSHHostKeyPolicyInsecure)
	}
	if (cfg.App.SSHHostKeyPolicy == SSHHostKeyPolicyAcceptNew ||
		cfg.App.SSHHostKeyPolicy == SSHHostKeyPolicyStrict) &&
		cfg.App.SSHKnownHosts == "" {
		return fmt.Errorf("app.ssh_known_hosts_file is required when app.ssh_host_key_policy is %q", cfg.App.SSHHostKeyPolicy)
	}
	if cfg.App.SSHKnownHosts != "" && !filepath.IsAbs(cfg.App.SSHKnownHosts) {
		return fmt.Errorf("app.ssh_known_hosts_file must be an absolute path")
	}

	keyIDs := map[string]bool{}
	for i, key := range cfg.Keys {
		if key.ID == "" {
			return fmt.Errorf("keys[%d]: id is required", i)
		}
		if keyIDs[key.ID] {
			return fmt.Errorf("keys[%d]: duplicate id %q", i, key.ID)
		}
		keyIDs[key.ID] = true
		if key.Name == "" {
			return fmt.Errorf("keys[%d] (%s): name is required", i, key.ID)
		}
		if key.File == "" {
			return fmt.Errorf("keys[%d] (%s): file is required", i, key.ID)
		}
		if filepath.Base(key.File) != key.File || strings.Contains(key.File, "..") {
			return fmt.Errorf("keys[%d] (%s): file must be a simple relative file name", i, key.ID)
		}
	}

	ids := map[string]bool{}
	for i, r := range cfg.Remotes {
		if r.ID == "" {
			return fmt.Errorf("remotes[%d]: id is required", i)
		}
		if ids[r.ID] {
			return fmt.Errorf("remotes[%d]: duplicate id %q", i, r.ID)
		}
		ids[r.ID] = true
		if r.Host == "" {
			return fmt.Errorf("remotes[%d] (%s): host is required", i, r.ID)
		}
		if r.Port <= 0 || r.Port > 65535 {
			return fmt.Errorf("remotes[%d] (%s): port must be 1-65535", i, r.ID)
		}
		if r.User == "" {
			return fmt.Errorf("remotes[%d] (%s): user is required", i, r.ID)
		}
		if r.KeyID != "" && !keyIDs[r.KeyID] {
			return fmt.Errorf("remotes[%d] (%s): key_id %q not found", i, r.ID, r.KeyID)
		}
	}

	tids := map[string]bool{}
	for i, t := range cfg.Tunnels {
		if t.ID == "" {
			return fmt.Errorf("tunnels[%d]: id is required", i)
		}
		if tids[t.ID] {
			return fmt.Errorf("tunnels[%d]: duplicate id %q", i, t.ID)
		}
		tids[t.ID] = true
		if !ids[t.RemoteID] {
			return fmt.Errorf("tunnels[%d] (%s): remote_id %q not found", i, t.ID, t.RemoteID)
		}
		if t.Direction != DirectionLocal && t.Direction != DirectionRemote {
			return fmt.Errorf("tunnels[%d] (%s): direction must be -L or -R", i, t.ID)
		}
		if t.BindPort <= 0 || t.BindPort > 65535 {
			return fmt.Errorf("tunnels[%d] (%s): bind_port must be 1-65535", i, t.ID)
		}
		if t.TargetHost == "" {
			return fmt.Errorf("tunnels[%d] (%s): target_host is required", i, t.ID)
		}
		if t.TargetPort <= 0 || t.TargetPort > 65535 {
			return fmt.Errorf("tunnels[%d] (%s): target_port must be 1-65535", i, t.ID)
		}
	}
	return nil
}

// ApplyDefaults fills in optional fields with their default values.
func ApplyDefaults(cfg *Config, defaultKnownHosts string) {
	if cfg.App.HTTPListen == "" {
		cfg.App.HTTPListen = "127.0.0.1:2222"
	}
	if cfg.App.LogLevel == "" {
		cfg.App.LogLevel = "info"
	}
	if cfg.App.SSHHostKeyPolicy == "" {
		cfg.App.SSHHostKeyPolicy = SSHHostKeyPolicyAcceptNew
	}
	if cfg.App.SSHKnownHosts == "" && defaultKnownHosts != "" {
		cfg.App.SSHKnownHosts = defaultKnownHosts
	}
	for i := range cfg.Remotes {
		if cfg.Remotes[i].Port == 0 {
			cfg.Remotes[i].Port = 22
		}
	}
	for i := range cfg.Tunnels {
		if cfg.Tunnels[i].BindAddress == "" {
			cfg.Tunnels[i].BindAddress = "127.0.0.1"
		}
	}
}
