//go:build !windows

package services

import (
	"os"
	"syscall"
)

// signalTerminate asks the process to shut down cleanly via SIGTERM. ssh
// responds by sending a disconnect message, which prompts the remote sshd to
// release any -R forwarded listening port right away instead of after a
// dead-connection timeout.
func signalTerminate(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
