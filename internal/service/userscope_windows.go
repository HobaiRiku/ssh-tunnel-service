//go:build windows

package service

import "errors"

// errUserScopeUnsupported is returned for `--user` operations on Windows, where
// the Service Control Manager has no per-user service concept. Users should run
// a system install (the default) or `ssh-tunnel run` for a foreground instance.
var errUserScopeUnsupported = errors.New(
	"user-level services are not supported on Windows; install the system service (default) or use `ssh-tunnel run`")

// checkUserScope rejects user-scope operations on Windows.
func checkUserScope(user bool) error {
	if user {
		return errUserScopeUnsupported
	}
	return nil
}
