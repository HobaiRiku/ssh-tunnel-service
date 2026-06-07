//go:build windows

package service

import (
	"os"
	"path/filepath"
)

// binaryDest installs the executable under %ProgramData% so the SCM service,
// running as LocalSystem, references a machine-wide path independent of the
// installing admin's profile.
func binaryDest() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = os.TempDir()
	}
	// Directory keyed by the app (ssh-tunnel-service); the executable is named
	// after the CLI (ssh-tunnel.exe), not the service registration name.
	return filepath.Join(base, "ssh-tunnel-service", "bin", "ssh-tunnel.exe")
}
