# ssh-tunnel-service

Cross-platform Go daemon that manages SSH `-L` / `-R` port-forwarding tunnels via a CLI, REST API, and an embedded Vue 3 web UI with interactive topology visualization.

## Features

- **Remotes** — reusable SSH server definitions (host, port, user)
- **Tunnels** — `-L` (local forward) or `-R` (remote forward) rules referencing a remote
- **YAML config** — all config driven by `~/.ssh-tunnel-service/config.yaml`
- **API token auth** — generated on first run; injected into the web UI automatically
- **Non-interactive SSH** — service-managed `known_hosts` trust store with configurable host-key policy
- **Visual topology** — interactive flow diagram showing tunnels, remotes, and port mappings, grouped by remote
- **Daemon management** — install/start/stop/uninstall as a system service (launchd, systemd, SCM)
- **AI-ready CLI** — structured JSON output for `remote list` and `tunnel list`
- **PWA** — installable as a progressive web app from the browser

## Installation

### Homebrew (macOS / Linux)

```bash
brew install hobairiku/tap/ssh-tunnel-service
```

### Go install

```bash
go install github.com/HobaiRiku/ssh-tunnel-service@latest
```

### Pre-built binaries

Download the appropriate binary from the [Releases](https://github.com/HobaiRiku/ssh-tunnel-service/releases) page, make it executable, and place it on your `$PATH`.

## Quick start

```bash
# Run in foreground (development)
go run . run

# Or with make
make dev
```

Open: `http://localhost:2222`

On first run the service generates an API token and writes it to `~/.ssh-tunnel-service/token` (printed to stderr). The web UI retrieves the token automatically; CLI commands also read it from the same file.

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

All API calls (except `/api/health` and `/api/bootstrap`) require the `Authorization: Bearer <token>` header. The token is stored in `~/.ssh-tunnel-service/token`:

```bash
TOKEN=$(cat ~/.ssh-tunnel-service/token)

# List remotes
curl -H "Authorization: Bearer $TOKEN" http://localhost:2222/api/remotes

# Add a remote
curl -X POST http://localhost:2222/api/remotes \
  -H "Authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"id":"prod","name":"Production","host":"ssh.example.com","port":22,"user":"ubuntu"}'

# Add a -L tunnel (local :15432 → remote postgres :5432)
curl -X POST http://localhost:2222/api/tunnels \
  -H "Authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' \
  -d '{"id":"db","name":"DB","remote_id":"prod","direction":"-L","bind_address":"127.0.0.1","bind_port":15432,"target_host":"127.0.0.1","target_port":5432,"auto_start":true}'

# Start / stop
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:2222/api/tunnels/db/start
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:2222/api/tunnels/db/stop
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

An example config is generated at the configured home path on first run. The `api_token` field is automatically populated on first startup.

Useful config helpers:

```bash
ssh-tunnel-service config path
ssh-tunnel-service config known-hosts-path
ssh-tunnel-service config show
```

### SSH host key policy

The service always starts `ssh` in non-interactive mode (`BatchMode=yes`, with password and keyboard-interactive auth disabled), so tunnel startup will either succeed immediately or fail with a diagnostic error instead of hanging on a host-key/password prompt.

### Authentication

Because tunnels run non-interactively, **remotes must authenticate with a key** — password and keyboard-interactive prompts are turned off. Load your key into the agent (`ssh-add`) or rely on the default identity files (`~/.ssh/id_*`). A remote that only accepts a password will fail fast and the tunnel is flagged as `error` with a diagnostic explaining that key-based authentication is required.

Supported `app.ssh_host_key_policy` values:

- `accept-new` — default; automatically trusts first-seen host keys, then enforces future verification
- `strict` — only trusts hosts already present in the configured `known_hosts` file
- `insecure` — disables host-key verification; use only for temporary debugging

`app.ssh_known_hosts_file` defaults to `<SSH_TUNNEL_HOME>/known_hosts`, so the service does not implicitly depend on the current user's `~/.ssh/known_hosts`.

`ssh_options` is still supported for advanced OpenSSH overrides, but the service now manages the default non-interactive and host-key behavior directly.
