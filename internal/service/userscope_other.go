//go:build !windows

package service

import "fmt"

// checkUserScope rejects user-scope operations on platforms that don't support
// them. On OpenWrt there is no user-session concept; use system install (default)
// or `ssh-tunnel run`. On other Unix platforms (Linux, macOS) user-level services
// are supported via systemd --user / LaunchAgent.
func checkUserScope(user bool) error {
	if user && isOpenWrt() {
		return fmt.Errorf("user-level services are not supported on OpenWrt; install the system service (omit --user) or use `ssh-tunnel run`")
	}
	return nil
}
