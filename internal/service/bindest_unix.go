//go:build !windows

package service

import (
	"fmt"
	"os"
	"path/filepath"
)

// binaryDest is the stable install path for the executable on Unix. The file is
// named after the CLI (`ssh-tunnel`), not the service registration name
// (`ssh-tunnel-service`).
//
//   - OpenWrt        → /usr/sbin/ssh-tunnel (procd convention; always system).
//   - system scope   → /usr/local/bin/ssh-tunnel (on PATH, root-writable, which
//     the elevated install guarantees).
//   - user scope     → ~/.local/bin/ssh-tunnel (user-writable, no root needed).
func binaryDest(user bool) (string, error) {
	if isOpenWrt() {
		return "/usr/sbin/ssh-tunnel", nil
	}
	if user {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve user home for binary install: %w", err)
		}
		return filepath.Join(home, ".local", "bin", "ssh-tunnel"), nil
	}
	return "/usr/local/bin/ssh-tunnel", nil
}
