//go:build !windows && !darwin

package elevate

// relaunch re-executes the binary as root via sudo. On a TTY sudo prompts for a
// password inline; in a non-interactive context it fails fast with sudo's own
// diagnostic, which is the desired behavior for scripts/CI. macOS overrides this
// to additionally support the graphical authentication dialog.
func relaunch() error {
	return sudoRelaunch()
}
