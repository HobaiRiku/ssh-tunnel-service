//go:build windows

package services

import "os"

// signalTerminate has no graceful equivalent on Windows — the runtime only
// supports Kill for child processes — so terminate the process directly. The
// caller still waits for it to exit before re-binding ports.
func signalTerminate(p *os.Process) error {
	return p.Kill()
}
