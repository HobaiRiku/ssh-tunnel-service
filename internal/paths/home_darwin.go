//go:build darwin

package paths

import (
	"os/user"
	"strconv"

	"ssh-tunnel-service/internal/elevate"
)

// systemHome is the default data root for the system-level LaunchDaemon on
// macOS. /Library/Application Support is the standard machine-wide location and
// is reachable by the daemon regardless of which admin ran install.
const systemHome = "/Library/Application Support/ssh-tunnel-service"

// fallbackAdminGID is the well-known GID of the macOS "admin" group, used when
// the group can't be looked up by name.
const fallbackAdminGID = 80

func platformDefaultHome() (string, error) {
	if elevate.IsElevated() {
		return systemHome, nil
	}
	return userHome()
}

// homePerm grants the macOS admin group access to the elevated system home so
// admin users can read and edit config.yaml directly (matching how most
// /Library/Application Support data is laid out: root:admin, 0755 dirs). The
// API token and private keys stay 0600 owner-only — admin members get the
// token over the loopback /api/bootstrap, never off disk. A per-user (session)
// instance lives under the user's own HOME and stays fully private.
func homePerm(string) Perm {
	if !elevate.IsElevated() {
		return privatePerm
	}
	return Perm{
		Dir:    0o755,
		Config: 0o664,
		Secret: 0o600,
		Group:  adminGID(),
	}
}

func adminGID() int {
	if g, err := user.LookupGroup("admin"); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			return gid
		}
	}
	return fallbackAdminGID
}
