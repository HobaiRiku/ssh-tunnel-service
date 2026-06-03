//go:build linux

package endpoint

import (
	"fmt"
	"os"
	"path/filepath"
)

const dirName = "ssh-tunnel-service"

// scopeDir returns the runtime directory for the given scope.
//
//	system → /run/ssh-tunnel-service          (tmpfs, cleared on boot)
//	user   → $XDG_RUNTIME_DIR/ssh-tunnel-service, falling back to
//	         /run/user/<uid>/ssh-tunnel-service, then a temp dir.
func scopeDir(scope Scope) (string, error) {
	if scope == ScopeSystem {
		return filepath.Join("/run", dirName), nil
	}
	if x := os.Getenv("XDG_RUNTIME_DIR"); x != "" {
		return filepath.Join(x, dirName), nil
	}
	if uid := os.Getuid(); uid >= 0 {
		candidate := filepath.Join("/run/user", fmt.Sprintf("%d", uid))
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return filepath.Join(candidate, dirName), nil
		}
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", dirName, os.Getuid())), nil
}
