package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installSystemBinary copies the current executable to the platform's stable
// system binary path (see binaryDest) and returns that path. If the running
// executable already *is* the destination — resolving symlinks on both sides so
// the check is reliable — it is a no-op and no copy occurs.
func installSystemBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve current executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dest := binaryDest()
	resolvedDest := dest
	if real, err := filepath.EvalSymlinks(dest); err == nil {
		resolvedDest = real
	}
	if filepath.Clean(exe) == filepath.Clean(resolvedDest) {
		return dest, nil
	}
	if err := copyExecutable(exe, dest); err != nil {
		return "", fmt.Errorf("install binary to %s: %w", dest, err)
	}
	return dest, nil
}

// removeSystemBinary deletes the installed binary if present.
func removeSystemBinary() error {
	dest := binaryDest()
	if err := os.Remove(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", dest, err)
	}
	return nil
}

// copyExecutable atomically copies src to dst (temp file + rename) with an
// executable mode.
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
