//go:build darwin

package endpoint

import (
	"os"
	"path/filepath"
)

const dirName = "ssh-tunnel-service"

// scopeDir returns the runtime directory for the given scope.
//
//	system → /var/run/ssh-tunnel-service
//	user   → ~/Library/Caches/ssh-tunnel-service (os.UserCacheDir), falling
//	         back to a temp dir.
func scopeDir(scope Scope) (string, error) {
	if scope == ScopeSystem {
		return filepath.Join("/var/run", dirName), nil
	}
	if cache, err := os.UserCacheDir(); err == nil && cache != "" {
		return filepath.Join(cache, dirName), nil
	}
	return filepath.Join(os.TempDir(), dirName), nil
}
