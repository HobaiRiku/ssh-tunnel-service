package config

import "os"

const exampleYAML = `# ~/.ssh-tunnel-service/config.yaml
# ssh-tunnel-service configuration

app:
  http_listen: "127.0.0.1:2222"   # management API + Web UI
  log_level: info                  # debug | info | warn | error
  log_console: false
  ssh_host_key_policy: accept-new  # accept-new | strict | insecure
  # ssh_known_hosts_file defaults to <SSH_TUNNEL_HOME>/known_hosts when empty

# ─── Remote SSH servers (reusable targets) ─────────────────────────────────
remotes:
  - id: example-remote
    name: "Example Server"
    host: ssh.example.com
    port: 22
    user: ubuntu
    description: "Example production SSH server"

# ─── Tunnels (ssh -L / -R definitions) ─────────────────────────────────────
tunnels:
  - id: example-local-forward
    name: "DB Local Forward"
    remote_id: example-remote
    direction: "-L"               # -L (local) or -R (remote)
    bind_address: "127.0.0.1"
    bind_port: 15432
    target_host: "127.0.0.1"
    target_port: 5432
    ssh_options:
      - "-o"
      - "ServerAliveInterval=30"
      # Advanced override; managed non-interactive/host-key options are added automatically.
    auto_start: false
    description: "Forward local :15432 → remote db :5432"

  - id: example-remote-forward
    name: "Web Remote Forward"
    remote_id: example-remote
    direction: "-R"
    bind_address: "0.0.0.0"
    bind_port: 8080
    target_host: "127.0.0.1"
    target_port: 3000
    auto_start: false
    description: "Expose local :3000 on remote :8080"
`

// WriteExample writes the annotated example config to path with the given mode.
func WriteExample(path string, mode os.FileMode) error {
	return WriteRaw(path, []byte(exampleYAML), mode)
}
