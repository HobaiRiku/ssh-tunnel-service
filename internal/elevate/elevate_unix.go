//go:build !windows

package elevate

import (
	"fmt"
	"os"
	"os/exec"
)

// IsElevated reports whether the effective user is root.
func IsElevated() bool { return os.Geteuid() == 0 }

// hasTTY reports whether stdin is attached to a terminal, in which case sudo can
// prompt for a password inline.
func hasTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// sudoRelaunch re-executes the current binary under sudo, preserving the
// environment (-E so SSH_TUNNEL_HOME survives) and forwarding all arguments plus
// any extra args the caller wants appended.
func sudoRelaunch(extraArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("this command requires root; re-run with sudo (sudo not found: %w)", err)
	}
	args := append([]string{"-E", exe}, os.Args[1:]...)
	args = append(args, extraArgs...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
