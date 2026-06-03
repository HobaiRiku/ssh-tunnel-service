//go:build !windows

package service

// binaryDest is the stable, system-wide install path for the executable on
// Unix. /usr/local/bin is on the default PATH and writable only by root, which
// the elevated install already guarantees.
func binaryDest() string { return "/usr/local/bin/ssh-tunnel-service" }
