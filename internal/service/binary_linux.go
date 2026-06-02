//go:build linux

package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const systemBinPath = "/usr/local/bin/ssh-tunnel-service"

// installSystemBinary copies the current executable to /usr/local/bin/ssh-tunnel-service
// and returns the destination path. If the current executable is already at the
// destination, it is a no-op.
func installSystemBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dest := systemBinPath
	if filepath.Clean(exe) == filepath.Clean(dest) {
		return dest, nil
	}
	if err := copyExecutable(exe, dest); err != nil {
		return "", fmt.Errorf("install binary to %s: %w", dest, err)
	}
	return dest, nil
}

// removeSystemBinary removes the installed binary if it exists.
func removeSystemBinary() error {
	err := os.Remove(systemBinPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", systemBinPath, err)
	}
	return nil
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
