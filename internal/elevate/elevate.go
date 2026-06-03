// Package elevate centralizes privilege detection and self-re-execution with
// elevated privileges. Service control commands (install/uninstall/start/stop)
// register and manage a system-level service and therefore require admin/root.
//
// The platform-specific files provide:
//   - IsElevated() bool  — is the current process already privileged?
//   - relaunch() error   — re-execute the current binary with elevation,
//     forwarding the same arguments and waiting for completion.
package elevate

import "errors"

// ErrDeclined is returned when the user cancels the elevation prompt.
var ErrDeclined = errors.New("privilege elevation was declined")

// Ensure guarantees the current process runs with the privileges required to
// manage a system service.
//
// It returns (relaunched, err):
//   - relaunched == false: the process is already elevated; the caller should
//     proceed to do the work itself.
//   - relaunched == true: the work has been (or attempted to be) performed by a
//     re-executed elevated child; the caller should return err as-is without
//     doing the work again.
func Ensure() (bool, error) {
	if IsElevated() {
		return false, nil
	}
	return true, relaunch()
}
