// Package paths resolves the SSH_TUNNEL_HOME root and the file/directory
// layout for ssh-tunnel-service.
//
// Resolution order (first hit wins):
//  1. explicit override (--home flag on the root cobra command)
//  2. SSH_TUNNEL_HOME environment variable
//  3. $HOME/.ssh-tunnel-service
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

	fileConfig     = "config.yaml"
	fileLog        = "ssh-tunnel-service.log"
	fileKnownHosts = "known_hosts"

	dirMode  os.FileMode = 0o700
	fileMode os.FileMode = 0o600
)

// Paths holds the resolved absolute root and helpers for well-known sub-locations.
type Paths struct {
	Home string
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
	return Paths{Home: abs}, nil
}

func pickHome(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env := os.Getenv(envHome); env != "" {
		return env, nil
	}
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "", errors.New("cannot determine user home; set SSH_TUNNEL_HOME or pass --home")
	}
	return filepath.Join(h, defaultDirName), nil
}

// EnsureTree creates Home, data/, logs/ with mode 0700, idempotent.
func (p Paths) EnsureTree() error {
	for _, d := range []string{p.Home, p.Data(), p.Logs()} {
		if err := os.MkdirAll(d, dirMode); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		if err := os.Chmod(d, dirMode); err != nil {
			return fmt.Errorf("chmod %s: %w", d, err)
		}
	}
	return nil
}

func (p Paths) Data() string          { return filepath.Join(p.Home, subData) }
func (p Paths) Logs() string          { return filepath.Join(p.Home, subLogs) }
func (p Paths) Config() string        { return filepath.Join(p.Home, fileConfig) }
func (p Paths) KnownHosts() string    { return filepath.Join(p.Home, fileKnownHosts) }
func (p Paths) LogFile() string       { return filepath.Join(p.Logs(), fileLog) }
func (p Paths) FileMode() os.FileMode { return fileMode }
