package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// GenerateToken returns a 32-byte cryptographically random hex string.
func GenerateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// MaxNameLength bounds resource names so they stay usable as URL path segments
// and table labels.
const MaxNameLength = 64

// ValidateName enforces the tightened rules for a resource name now that the
// name is the unique identifier: non-empty, no surrounding whitespace, no path
// separators or control characters, and a sane length cap. kind is used only to
// build a readable error message (e.g. "tunnel", "remote", "key").
func ValidateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("%s name %q must not have leading or trailing whitespace", kind, name)
	}
	if !utf8.ValidString(name) {
		return fmt.Errorf("%s name must be valid UTF-8", kind)
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return fmt.Errorf("%s name must be at most %d characters", kind, MaxNameLength)
	}
	for _, r := range name {
		if r == '/' || r == '\\' {
			return fmt.Errorf("%s name %q must not contain '/' or '\\'", kind, name)
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%s name %q must not contain control characters", kind, name)
		}
	}
	return nil
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

	keyNames := map[string]bool{}
	for i, key := range cfg.Keys {
		if err := ValidateName("key", key.Name); err != nil {
			return fmt.Errorf("keys[%d]: %w", i, err)
		}
		if keyNames[key.Name] {
			return fmt.Errorf("keys[%d]: duplicate name %q", i, key.Name)
		}
		keyNames[key.Name] = true
		if key.File == "" {
			return fmt.Errorf("keys[%d] (%s): file is required", i, key.Name)
		}
		if filepath.Base(key.File) != key.File || strings.Contains(key.File, "..") {
			return fmt.Errorf("keys[%d] (%s): file must be a simple relative file name", i, key.Name)
		}
	}

	remoteNames := map[string]bool{}
	for i, r := range cfg.Remotes {
		if err := ValidateName("remote", r.Name); err != nil {
			return fmt.Errorf("remotes[%d]: %w", i, err)
		}
		if remoteNames[r.Name] {
			return fmt.Errorf("remotes[%d]: duplicate name %q", i, r.Name)
		}
		remoteNames[r.Name] = true
		if r.Host == "" {
			return fmt.Errorf("remotes[%d] (%s): host is required", i, r.Name)
		}
		if r.Port <= 0 || r.Port > 65535 {
			return fmt.Errorf("remotes[%d] (%s): port must be 1-65535", i, r.Name)
		}
		if r.User == "" {
			return fmt.Errorf("remotes[%d] (%s): user is required", i, r.Name)
		}
		if r.Key != "" && !keyNames[r.Key] {
			return fmt.Errorf("remotes[%d] (%s): key %q not found", i, r.Name, r.Key)
		}
	}

	tunnelNames := map[string]bool{}
	for i, t := range cfg.Tunnels {
		if err := ValidateName("tunnel", t.Name); err != nil {
			return fmt.Errorf("tunnels[%d]: %w", i, err)
		}
		if tunnelNames[t.Name] {
			return fmt.Errorf("tunnels[%d]: duplicate name %q", i, t.Name)
		}
		tunnelNames[t.Name] = true
		if !remoteNames[t.Remote] {
			return fmt.Errorf("tunnels[%d] (%s): remote %q not found", i, t.Name, t.Remote)
		}
		if t.Direction != DirectionLocal && t.Direction != DirectionRemote {
			return fmt.Errorf("tunnels[%d] (%s): direction must be -L or -R", i, t.Name)
		}
		if t.BindPort <= 0 || t.BindPort > 65535 {
			return fmt.Errorf("tunnels[%d] (%s): bind_port must be 1-65535", i, t.Name)
		}
		if t.TargetHost == "" {
			return fmt.Errorf("tunnels[%d] (%s): target_host is required", i, t.Name)
		}
		if t.TargetPort <= 0 || t.TargetPort > 65535 {
			return fmt.Errorf("tunnels[%d] (%s): target_port must be 1-65535", i, t.Name)
		}
	}
	return nil
}

// ApplyDefaults fills in optional fields with their default values.
func ApplyDefaults(cfg *Config, defaultKnownHosts string) {
	if cfg.App.HTTPListen == "" {
		cfg.App.HTTPListen = DefaultHTTPListen
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
