# ssh-tunnel-service

Cross-platform Go service that manages SSH `-L` / `-R` port-forwarding tunnels. Follows the [ws2tcp](https://github.com/HobaiRiku/ws2tcp) architecture: cobra CLI, kardianos/service daemon, gin HTTP API, and an embedded Vue 3 SPA with interactive topology visualization.

## Features

- **Remotes** — reusable SSH server definitions (host, port, user)
- **Tunnels** — `-L` (local forward) or `-R` (remote forward) rules referencing a remote
- **YAML config** — all config driven by `~/.ssh-tunnel-service/config.yaml`
- **Non-interactive SSH** — service-managed `known_hosts` trust store with configurable host-key policy
- **Visual topology** — Vue Flow diagram showing tunnels, remotes, and data-flow direction
- **Daemon management** — install/start/stop/uninstall as a system service (launchd, systemd, SCM)
- **AI-ready CLI** — structured JSON output for `remote list` and `tunnel list`

## Quick start

```bash
# Run in foreground (development)
go run . run

# Or with make
make dev
```

Open: `http://localhost:2222`

## CLI reference

```
ssh-tunnel-service [command]

Commands:
  run         Run the service in the foreground
  install     Install as a system service
  uninstall   Remove the system service registration
  start       Start the installed service
  stop        Stop the installed service
  status      Show service status

  remote      Manage remote SSH targets
    list      List remotes (--json for machine-readable output)
    add       Add a remote  (--id, --name, --host required)
    update    Update a remote field
    rm        Remove a remote

  tunnel      Manage SSH tunnel definitions
    list      List tunnels and live state
    add       Add a tunnel (--id, --name, --remote, --bind-port, --target-host, --target-port required)
    update    Update a tunnel field
    rm        Remove a tunnel
    start     Start a tunnel via the running service
    stop      Stop a running tunnel

  config      Manage configuration
    show      Print config as YAML
    edit      Open config in $EDITOR
    path      Print config file path
    known-hosts-path  Print the effective managed known_hosts path

  version     Print version information
```

## API examples

```bash
# List remotes
curl http://localhost:2222/api/remotes

# Add a remote
curl -X POST http://localhost:2222/api/remotes \
  -H 'content-type: application/json' \
  -d '{"id":"prod","name":"Production","host":"ssh.example.com","port":22,"user":"ubuntu"}'

# Add a -L tunnel (local :15432 → remote postgres :5432)
curl -X POST http://localhost:2222/api/tunnels \
  -H 'content-type: application/json' \
  -d '{"id":"db","name":"DB","remote_id":"prod","direction":"-L","bind_address":"127.0.0.1","bind_port":15432,"target_host":"127.0.0.1","target_port":5432,"auto_start":true}'

# Start / stop
curl -X POST http://localhost:2222/api/tunnels/db/start
curl -X POST http://localhost:2222/api/tunnels/db/stop

# Topology graph (Vue Flow-compatible nodes/edges)
curl http://localhost:2222/api/topology
```

## Building

```bash
# Backend only (placeholder UI)
go build .

# Full release binary with embedded SPA
make build

# Cross-platform distribution matrix
make dist
```

## Web UI development

```bash
make ui-dev     # Vite dev server on :5173 with /api proxy to :2222
make ui-build   # Build SPA into internal/web/static/
# If pnpm is not installed globally, prefix with `corepack`
# corepack pnpm install
```

## Configuration

Default location: `~/.ssh-tunnel-service/config.yaml`  
Override: `SSH_TUNNEL_HOME=/path/to/dir` or `--home /path/to/dir`

An example config is generated at the configured home path on first run.

Useful config helpers:

```bash
ssh-tunnel-service config path
ssh-tunnel-service config known-hosts-path
ssh-tunnel-service config show
```

### SSH host key policy

The service always starts `ssh` in non-interactive mode (`BatchMode=yes`), so tunnel startup will either succeed immediately or fail with a diagnostic error instead of hanging on a host-key/password prompt.

Supported `app.ssh_host_key_policy` values:

- `accept-new` — default; automatically trusts first-seen host keys, then enforces future verification
- `strict` — only trusts hosts already present in the configured `known_hosts` file
- `insecure` — disables host-key verification; use only for temporary debugging

`app.ssh_known_hosts_file` defaults to `<SSH_TUNNEL_HOME>/known_hosts`, so the service does not implicitly depend on the current user's `~/.ssh/known_hosts`.

`ssh_options` is still supported for advanced OpenSSH overrides, but the service now manages the default non-interactive and host-key behavior directly.
