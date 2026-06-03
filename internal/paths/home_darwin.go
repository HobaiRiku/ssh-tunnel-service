//go:build darwin

package paths

import "ssh-tunnel-service/internal/elevate"

// systemHome is the default data root for the system-level LaunchDaemon on
// macOS. /Library/Application Support is the standard machine-wide location and
// is reachable by the daemon regardless of which admin ran install.
const systemHome = "/Library/Application Support/ssh-tunnel-service"

func platformDefaultHome() (string, error) {
	if elevate.IsElevated() {
		return systemHome, nil
	}
	return userHome()
}
