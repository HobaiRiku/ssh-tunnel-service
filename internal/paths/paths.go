// Package paths resolves the SSH_TUNNEL_HOME root and the file/directory
// layout for ssh-tunnel-service.
//
// Resolution order (first hit wins):
//  1. explicit override (--home flag on the root cobra command)
//  2. SSH_TUNNEL_HOME environment variable
//  3. platform default — when running elevated this is a system location
//     (Linux: /etc/ssh-tunnel-service, macOS: /Library/Application Support/
//     ssh-tunnel-service, Windows: %ProgramData%\ssh-tunnel-service) so the
//     system service is independent of the installing user's HOME; otherwise it
//     is the per-user $HOME/.ssh-tunnel-service.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultDirName = ".ssh-tunnel-service"
	envHome        = "SSH_TUNNEL_HOME"

	subData = "data"
	subLogs = "logs"
	subKeys = "keys"

	fileConfig     = "config.yaml"
	fileLog        = "ssh-tunnel-service.log"
	fileKnownHosts = "known_hosts"
	fileToken      = "token"

	privateDirMode  os.FileMode = 0o700
	privateFileMode os.FileMode = 0o600
)

// Perm describes the ownership/permission model applied to a resolved home tree.
//
//   - Dir is the mode for the home and its sub-directories.
//   - Config is the mode for the editable config.yaml.
//   - Secret is the mode for the API token and private keys (never relaxed).
//   - Group is a numeric GID applied to the tree, or -1 to leave ownership as-is.
//
// The private model (everything owner-only, no chown) is the default on every
// platform. macOS overrides it for the elevated system service so members of
// the admin group can read and edit config.yaml directly — see home_darwin.go.
type Perm struct {
	Dir    os.FileMode
	Config os.FileMode
	Secret os.FileMode
	Group  int
}

var privatePerm = Perm{
	Dir:    privateDirMode,
	Config: privateFileMode,
	Secret: privateFileMode,
	Group:  -1,
}

// Paths holds the resolved absolute root and helpers for well-known sub-locations.
type Paths struct {
	Home string
	perm Perm
}

// perms returns the resolved permission model, falling back to the owner-only
// private defaults when Paths was constructed directly (zero-value perm, e.g. in
// tests) rather than via Resolve. A zero Dir is the tell-tale of an unset perm —
// without this, EnsureTree/FileMode/ConfigMode would yield mode 0000.
func (p Paths) perms() Perm {
	if p.perm.Dir == 0 {
		return privatePerm
	}
	return p.perm
}

// Resolve returns Paths honoring the precedence above. override may be empty.
func Resolve(override string) (Paths, error) {
	home, err := pickHome(override)
	if err != nil {
		return Paths{}, err
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home %q: %w", home, err)
	}
	return Paths{Home: abs, perm: homePerm(abs)}, nil
}

func pickHome(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv(envHome); env != "" {
		return env, nil
	}
	return platformDefaultHome()
}

// userHome returns the per-user data root, shared by every platform's
// non-elevated default.
func userHome() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "", errors.New("cannot determine user home; set SSH_TUNNEL_HOME or pass --home")
	}
	return filepath.Join(h, defaultDirName), nil
}

// EnsureTree creates Home, data/, logs/, keys/ with the resolved directory mode
// (0700 by default; 0755 root:admin for the macOS system service), idempotent.
func (p Paths) EnsureTree() error {
	pm := p.perms()
	for _, d := range []string{p.Home, p.Data(), p.Logs(), p.Keys()} {
		if err := os.MkdirAll(d, pm.Dir); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		if err := os.Chmod(d, pm.Dir); err != nil {
			return fmt.Errorf("chmod %s: %w", d, err)
		}
		if pm.Group >= 0 {
			if err := os.Chown(d, -1, pm.Group); err != nil {
				return fmt.Errorf("chown %s: %w", d, err)
			}
		}
	}
	// Repair an existing config.yaml so admin users can edit it after an upgrade
	// from the private (0600 owner-only) layout. New configs already land with
	// the right mode/group via config.WriteRaw inside the group-owned home.
	if pm.Group >= 0 {
		if err := relaxSharedFile(p.Config(), pm.Config, pm.Group); err != nil {
			return err
		}
	}
	return nil
}

// relaxSharedFile applies the shared mode and group to path when it exists,
// leaving a missing file untouched (it will be created with the right perms).
func relaxSharedFile(path string, mode os.FileMode, group int) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	if err := os.Chown(path, -1, group); err != nil {
		return fmt.Errorf("chown %s: %w", path, err)
	}
	return nil
}

func (p Paths) Data() string       { return filepath.Join(p.Home, subData) }
func (p Paths) Logs() string       { return filepath.Join(p.Home, subLogs) }
func (p Paths) Keys() string       { return filepath.Join(p.Home, subKeys) }
func (p Paths) Config() string     { return filepath.Join(p.Home, fileConfig) }
func (p Paths) KnownHosts() string { return filepath.Join(p.Home, fileKnownHosts) }
func (p Paths) Token() string      { return filepath.Join(p.Home, fileToken) }
func (p Paths) LogFile() string    { return filepath.Join(p.Logs(), fileLog) }

// FileMode is the mode for secrets (API token, private keys): always owner-only.
func (p Paths) FileMode() os.FileMode { return p.perms().Secret }

// ConfigMode is the mode for config.yaml — relaxed to be admin-group editable on
// the macOS system service, owner-only elsewhere.
func (p Paths) ConfigMode() os.FileMode { return p.perms().Config }
