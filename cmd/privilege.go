package cmd

import (
	"errors"
	"fmt"

	"ssh-tunnel-service/internal/elevate"
)

// ensurePrivileged guarantees the service-control commands run with the
// privileges required to manage a system-level service. When the current
// process is unprivileged it re-executes the same command with elevation
// (sudo / macOS auth dialog / Windows UAC) and reports the result.
//
// It returns (handled, err):
//   - handled == false: already privileged; the caller proceeds to do the work.
//   - handled == true:  an elevated child performed the work; the caller must
//     return err verbatim without repeating it.
func ensurePrivileged() (bool, error) {
	relaunched, err := elevate.Ensure()
	if err != nil {
		if errors.Is(err, elevate.ErrDeclined) {
			return true, fmt.Errorf("this command needs administrator privileges, but elevation was declined")
		}
		return true, err
	}
	return relaunched, nil
}
