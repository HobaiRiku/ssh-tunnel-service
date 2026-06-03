//go:build linux

package paths

import "ssh-tunnel-service/internal/elevate"

// systemHome is the default data root for the system-level service on Linux.
// Using /etc avoids the /root ownership problem when systemd runs the service
// without a usable HOME, and keeps the path independent of whoever ran install.
const systemHome = "/etc/ssh-tunnel-service"

func platformDefaultHome() (string, error) {
	if elevate.IsElevated() {
		return systemHome, nil
	}
	return userHome()
}
