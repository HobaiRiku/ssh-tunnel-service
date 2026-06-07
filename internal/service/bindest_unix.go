//go:build !windows

package service

// binaryDest is the stable, system-wide install path for the executable on
// Unix. /usr/local/bin is on the default PATH and writable only by root, which
// the elevated install already guarantees. The file is named after the CLI
// (`ssh-tunnel`), not the service registration name (`ssh-tunnel-service`).
func binaryDest() string { return "/usr/local/bin/ssh-tunnel" }
