// Command ssh-tunnel-service manages SSH -L/-R tunnels as a cross-platform service.
package main

import (
"fmt"
"os"

"github.com/HobaiRiku/ssh-tunnel-service/cmd"
"github.com/HobaiRiku/ssh-tunnel-service/internal/service"
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
