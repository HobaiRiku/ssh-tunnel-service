//go:build linux

package paths

// systemHome is the default data root for system-level service installs on Linux.
// Using /etc avoids the /root ownership problem when systemd runs with a limited HOME.
const systemHome = "/etc/ssh-tunnel-service"

func platformDefaultHome() (string, error) {
	return systemHome, nil
}
