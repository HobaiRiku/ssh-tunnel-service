//go:build !windows

package service

// checkUserScope is a no-op on platforms that support user-level services
// (Linux systemd --user, macOS LaunchAgent).
func checkUserScope(bool) error { return nil }
