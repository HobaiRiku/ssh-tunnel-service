package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// The schema dropped the per-resource `id` field in favour of using `name` as
// the unique key. Existing configs on disk still carry `id`, `remote_id` and
// `key_id`, so loading goes through a compatibility layer that reads both the
// legacy and current field names and rewrites everything to be name-keyed.
//
// Migration is best-effort and tolerant: missing/blank/duplicate names are
// repaired (the "兜底" fallback) so an older or hand-edited config still loads.
// Once a migrated config is persisted it is already name-keyed, and re-loading
// it is a no-op because the legacy fields are absent.

type compatKey struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	File        string `yaml:"file"`
	Description string `yaml:"description"`
}

type compatRemote struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	User        string `yaml:"user"`
	KeyID       string `yaml:"key_id"` // legacy reference (key id)
	Key         string `yaml:"key"`    // current reference (key name)
	Description string `yaml:"description"`
}

type compatTunnel struct {
	ID          string          `yaml:"id"`
	Name        string          `yaml:"name"`
	RemoteID    string          `yaml:"remote_id"` // legacy reference (remote id)
	Remote      string          `yaml:"remote"`    // current reference (remote name)
	Direction   TunnelDirection `yaml:"direction"`
	BindAddress string          `yaml:"bind_address"`
	BindPort    int             `yaml:"bind_port"`
	TargetHost  string          `yaml:"target_host"`
	TargetPort  int             `yaml:"target_port"`
	SSHOptions  []string        `yaml:"ssh_options"`
	AutoStart   bool            `yaml:"auto_start"`
	Description string          `yaml:"description"`
}

type compatConfig struct {
	App     AppConfig      `yaml:"app"`
	Keys    []compatKey    `yaml:"keys"`
	Remotes []compatRemote `yaml:"remotes"`
	Tunnels []compatTunnel `yaml:"tunnels"`
}

// parseConfig unmarshals raw YAML and migrates it to the canonical name-keyed
// schema.
func parseConfig(data []byte) (*Config, error) {
	var raw compatConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw.canonical(), nil
}

func (c compatConfig) canonical() *Config {
	out := &Config{App: c.App}

	keyNames := newNameAllocator()
	keyIDToName := map[string]string{}
	for i, k := range c.Keys {
		name := keyNames.allocate(pickName(k.Name, k.ID, "key", i))
		if k.ID != "" {
			keyIDToName[k.ID] = name
		}
		out.Keys = append(out.Keys, SSHKey{Name: name, File: k.File, Description: k.Description})
	}

	remoteNames := newNameAllocator()
	remoteIDToName := map[string]string{}
	for i, r := range c.Remotes {
		name := remoteNames.allocate(pickName(r.Name, r.ID, "remote", i))
		if r.ID != "" {
			remoteIDToName[r.ID] = name
		}
		ref := r.Key
		if ref == "" {
			ref = r.KeyID
		}
		if mapped, ok := keyIDToName[ref]; ok {
			ref = mapped
		}
		out.Remotes = append(out.Remotes, Remote{
			Name: name, Host: r.Host, Port: r.Port, User: r.User, Key: ref, Description: r.Description,
		})
	}

	tunnelNames := newNameAllocator()
	for i, t := range c.Tunnels {
		name := tunnelNames.allocate(pickName(t.Name, t.ID, "tunnel", i))
		ref := t.Remote
		if ref == "" {
			ref = t.RemoteID
		}
		if mapped, ok := remoteIDToName[ref]; ok {
			ref = mapped
		}
		out.Tunnels = append(out.Tunnels, Tunnel{
			Name: name, Remote: ref, Direction: t.Direction,
			BindAddress: t.BindAddress, BindPort: t.BindPort,
			TargetHost: t.TargetHost, TargetPort: t.TargetPort,
			SSHOptions: t.SSHOptions, AutoStart: t.AutoStart, Description: t.Description,
		})
	}

	return out
}

// pickName chooses the best candidate name for a migrated entity: the explicit
// name, then the legacy id, then a generated "<kind>-<n>" fallback.
func pickName(name, id, kind string, index int) string {
	if n := sanitizeName(name); n != "" {
		return n
	}
	if n := sanitizeName(id); n != "" {
		return n
	}
	return fmt.Sprintf("%s-%d", kind, index+1)
}

// sanitizeName coerces an arbitrary string into a value that passes
// ValidateName: trimmed, no path separators or control characters, capped
// length. It returns "" if nothing usable remains.
func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '/' || r == '\\':
			b.WriteByte('-')
		case unicode.IsControl(r):
			// drop
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(out) > MaxNameLength {
		out = string([]rune(out)[:MaxNameLength])
		out = strings.TrimSpace(out)
	}
	return out
}

// nameAllocator hands out unique names, appending "-2", "-3", … on collision.
type nameAllocator struct{ taken map[string]bool }

func newNameAllocator() *nameAllocator { return &nameAllocator{taken: map[string]bool{}} }

func (a *nameAllocator) allocate(base string) string {
	if base == "" {
		base = "item"
	}
	// Reserve room for a numeric suffix within the length cap.
	if utf8.RuneCountInString(base) > MaxNameLength-4 {
		base = string([]rune(base)[:MaxNameLength-4])
	}
	candidate := base
	for i := 2; a.taken[candidate]; i++ {
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	a.taken[candidate] = true
	return candidate
}
