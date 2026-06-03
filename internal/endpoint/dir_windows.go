//go:build windows

package endpoint

import (
	"os"
	"path/filepath"
)

const dirName = "ssh-tunnel-service"

// scopeDir returns the runtime directory for the given scope.
//
//	system → %ProgramData%\ssh-tunnel-service   (machine-wide, LocalSystem-readable)
//	user   → %LOCALAPPDATA%\ssh-tunnel-service
func scopeDir(scope Scope) (string, error) {
	if scope == ScopeSystem {
		if pd := os.Getenv("ProgramData"); pd != "" {
			return filepath.Join(pd, dirName), nil
		}
		return filepath.Join(os.TempDir(), dirName), nil
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, dirName), nil
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		return filepath.Join(cache, dirName), nil
	}
	return filepath.Join(os.TempDir(), dirName), nil
}
