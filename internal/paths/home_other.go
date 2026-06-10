//go:build !linux && !darwin && !windows

package paths

// platformDefaultHome falls back to the per-user directory on platforms without
// a dedicated system service model.
func platformDefaultHome() (string, error) {
	return userHome()
}

// homePerm keeps the home owner-only on platforms without a system service model.
func homePerm(string) Perm { return privatePerm }
