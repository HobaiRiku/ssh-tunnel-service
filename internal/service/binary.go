package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// installSystemBinary copies the current executable to the platform's stable
// system binary path (see binaryDest) and returns that path and whether a new
// copy was created. If the running executable already *is* the destination —
// resolving symlinks on both sides so the check is reliable — it is a no-op
// and created is false.
func installSystemBinary(user bool) (dest string, created bool, err error) {
	exe, err := os.Executable()
	if err != nil {
		return "", false, fmt.Errorf("resolve current executable: %w", err)
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dest, err = binaryDest(user)
	if err != nil {
		return "", false, err
	}
	resolvedDest := dest
	if real, err := filepath.EvalSymlinks(dest); err == nil {
		resolvedDest = real
	}
	if filepath.Clean(exe) == filepath.Clean(resolvedDest) {
		return dest, false, nil
	}
	if err := copyExecutable(exe, dest); err != nil {
		return "", false, fmt.Errorf("install binary to %s: %w", dest, err)
	}
	return dest, true, nil
}

// removeSystemBinary deletes the installed binary if present.
func removeSystemBinary(user bool) error {
	dest, err := binaryDest(user)
	if err != nil {
		return err
	}
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
