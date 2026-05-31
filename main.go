// Command ssh-tunnel manages SSH -L/-R tunnels as a cross-platform service.
package main

import (
	"fmt"
	"os"

	"ssh-tunnel-service/cmd"
	"ssh-tunnel-service/internal/service"
)

func main() {
	// When launched by an OS service manager (launchd, systemd, SCM) run in managed mode.
	if !service.Interactive() {
		if err := service.RunService(""); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
