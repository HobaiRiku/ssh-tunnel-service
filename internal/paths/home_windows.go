//go:build windows

package paths

import (
	"os"
	"path/filepath"

	"ssh-tunnel-service/internal/elevate"
)

// platformDefaultHome returns %ProgramData%\ssh-tunnel-service when elevated —
// the machine-wide data location every service account (including LocalSystem)
// can access — and the per-user directory otherwise.
func platformDefaultHome() (string, error) {
	if elevate.IsElevated() {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, "ssh-tunnel-service"), nil
		}
	}
	return userHome()
}
