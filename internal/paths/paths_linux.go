//go:build linux

package paths

import (
	"errors"
	"os"
	"path/filepath"
)

// systemHome is the default data root for system-level service installs on Linux.
// Using /etc avoids the /root ownership problem when systemd runs without a user HOME.
const systemHome = "/etc/ssh-tunnel-service"

// platformDefaultHome returns /etc/ssh-tunnel-service when the process is root
// (uid 0), which is the expected context for systemd service install/run.
// Non-root invocations fall back to $HOME/.ssh-tunnel-service so that
// unprivileged CLI commands (tail, config path, tunnel list, …) work without
// requiring --home or SSH_TUNNEL_HOME.
func platformDefaultHome() (string, error) {
	if os.Getuid() == 0 {
		return systemHome, nil
	}
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "", errors.New("cannot determine user home; set SSH_TUNNEL_HOME or pass --home")
	}
	return filepath.Join(h, defaultDirName), nil
}
