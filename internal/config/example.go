package config

import "os"

const exampleYAML = `# ~/.ssh-tunnel-service/config.yaml
# ssh-tunnel-service configuration
#
# IMPORTANT: tunnels are launched non-interactively (ssh BatchMode), so every
# remote MUST authenticate with a key — password / keyboard-interactive prompts
# are disabled. A remote that only accepts a password will fail fast and the
# tunnel is flagged as "error" rather than hanging. Add your key to the agent
# (ssh-add) or rely on the default identity files (~/.ssh/id_*).

app:
  http_listen: "127.0.0.1:2222"   # management API + Web UI
  log_level: info                  # debug | info | warn | error
  log_console: false
  ssh_host_key_policy: accept-new  # accept-new | strict | insecure
  # ssh_known_hosts_file defaults to <SSH_TUNNEL_HOME>/known_hosts when empty

# ─── Remote SSH servers (reusable targets) ─────────────────────────────────
remotes:
  - id: bastion
    name: "Production Bastion"
    host: bastion.prod.example.com
    port: 22
    user: deploy
    description: "Public jump host fronting the production VPC"

  - id: analytics-box
    name: "Analytics Host"
    host: 10.0.12.34
    port: 2222
    user: analyst
    description: "Internal analytics server (non-standard sshd port)"

# ─── Tunnels (ssh -L / -R definitions) ─────────────────────────────────────
tunnels:
  - id: postgres-forward
    name: "Postgres via Bastion"
    remote_id: bastion
    direction: "-L"               # -L (local) or -R (remote)
    bind_address: "127.0.0.1"
    bind_port: 15432
    target_host: "db.internal"
    target_port: 5432
    ssh_options:
      - "-o"
      - "ServerAliveInterval=30"
      - "-o"
      - "ServerAliveCountMax=3"
      # Advanced supplement; managed non-interactive/host-key options are still added automatically.
    auto_start: true
    description: "Reach the private Postgres at db.internal:5432 on localhost:15432"

  - id: grafana-forward
    name: "Grafana via Bastion"
    remote_id: bastion
    direction: "-L"
    bind_address: "127.0.0.1"
    bind_port: 3000
    target_host: "grafana.internal"
    target_port: 3000
    auto_start: false
    description: "Reach the internal Grafana UI on localhost:3000"

  - id: webhook-receiver
    name: "Expose Local Webhook Receiver"
    remote_id: analytics-box
    direction: "-R"
    bind_address: "127.0.0.1"
    bind_port: 9000
    target_host: "127.0.0.1"
    target_port: 8000
    auto_start: false
    description: "Publish the local dev webhook receiver (:8000) on the analytics host :9000"
`

// WriteExample writes the annotated example config to path with the given mode.
func WriteExample(path string, mode os.FileMode) error {
	return WriteRaw(path, []byte(exampleYAML), mode)
}
