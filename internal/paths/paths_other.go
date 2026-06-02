//go:build !linux

package paths

import (
	"errors"
	"os"
	"path/filepath"
)

func platformDefaultHome() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil || h == "" {
		return "", errors.New("cannot determine user home; set SSH_TUNNEL_HOME or pass --home")
	}
	return filepath.Join(h, defaultDirName), nil
}
