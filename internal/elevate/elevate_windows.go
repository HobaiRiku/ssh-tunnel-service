//go:build windows

package elevate

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process token is a member of the
// built-in Administrators group with elevation in effect.
func IsElevated() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	// Token(0) evaluates membership against the calling thread's effective token,
	// which reflects the actual elevation state under UAC.
	member, err := windows.Token(0).IsMember(sid)
	return err == nil && member
}

// relaunch re-executes the binary via ShellExecute with the "runas" verb, which
// triggers the UAC consent dialog. UAC always spawns a fresh elevated process in
// a new window; we therefore cannot stream its output back here, so the elevated
// instance is responsible for surfacing its own result.
func relaunch(extraArgs []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	allArgs := append(append([]string{}, os.Args[1:]...), extraArgs...)
	escaped := make([]string, len(allArgs))
	for i, a := range allArgs {
		escaped[i] = syscall.EscapeArg(a)
	}
	args, _ := windows.UTF16PtrFromString(strings.Join(escaped, " "))
	cwd, _ := windows.UTF16PtrFromString("")

	const swShowNormal = 1
	if err := windows.ShellExecute(0, verb, file, args, cwd, swShowNormal); err != nil {
		if err == windows.ERROR_CANCELLED {
			return ErrDeclined
		}
		return fmt.Errorf("UAC elevation failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "ssh-tunnel: continuing in an elevated window…")
	return nil
}
