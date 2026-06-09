package config

import (
	"os"
	"strings"
)

const exampleYAML = `# ssh-tunnel-service configuration
#
# IMPORTANT: tunnels are launched non-interactively (ssh BatchMode), so every
# remote MUST authenticate with a key — password / keyboard-interactive prompts
# are disabled. A remote that only accepts a password will fail fast and the
# tunnel is flagged as "error" rather than hanging.
#
# Each key, remote and tunnel is identified by its unique "name". Remotes
# reference a key by name; tunnels reference their remote by name. Manage them
# through the CLI (` + "`ssh-tunnel key/remote/tunnel ...`" + `) or the Web UI —
# this file starts empty on purpose.
#
# When running as a system service, tunnels whose remote binds no key use the
# managed key named by app.system_default_key (generated automatically on first
# system-service start). In a per-user "ssh-tunnel run", unbound tunnels instead
# rely on your normal ssh defaults (ssh-agent / ~/.ssh/id_*).

app:
  http_listen: "127.0.0.1:2222"   # management API + Web UI
  # The log level is fixed at "info" and is not configurable.
  log_console: false               # also mirror logs to stderr (foreground runs)
  # Log file rotation / retention (the active log is <SSH_TUNNEL_HOME>/logs/):
  log_max_size_mb: 20              # rotate the log once it reaches this size
  log_max_backups: 10              # number of rotated files to keep
  log_max_age_days: 14             # days to preserve rotated files before deleting
  log_compress: false              # gzip rotated files
  ssh_host_key_policy: accept-new  # accept-new | strict | insecure
  # ssh_known_hosts_file defaults to <SSH_TUNNEL_HOME>/known_hosts when empty
  # system_default_key is set automatically on first system-service start

keys: []

remotes: []

tunnels: []
`

// WriteExample writes the annotated example config to path with the given mode,
// binding the management API to DefaultHTTPListen.
func WriteExample(path string, mode os.FileMode) error {
	return WriteExampleListening(path, mode, DefaultHTTPListen)
}

// WriteExampleListening writes the annotated example config to path, binding
// the management API to listen instead of the default address. Used to seed a
// fresh install with a port that doesn't collide with another instance already
// running on the host (see PickAvailableListen).
func WriteExampleListening(path string, mode os.FileMode, listen string) error {
	yaml := strings.Replace(exampleYAML, `"`+DefaultHTTPListen+`"`, `"`+listen+`"`, 1)
	return WriteRaw(path, []byte(yaml), mode)
}
