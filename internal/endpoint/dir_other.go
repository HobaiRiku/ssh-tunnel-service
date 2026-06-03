//go:build !linux && !darwin && !windows

package endpoint

import (
	"os"
	"path/filepath"
)

const dirName = "ssh-tunnel-service"

func scopeDir(scope Scope) (string, error) {
	if scope == ScopeSystem {
		return filepath.Join("/var/run", dirName), nil
	}
	return filepath.Join(os.TempDir(), dirName), nil
}
