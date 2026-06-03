# ssh-tunnel-service

Cross-platform Go daemon that manages SSH `-L` / `-R` port-forwarding tunnels via a CLI, REST API, and an embedded Vue 3 web UI with interactive topology, managed SSH keys, and English/中文 UI switching.

## Features

- **Remotes** — reusable SSH server definitions (host, port, user, optional managed key)
- **Keys** — paste or upload private keys into the runtime config directory, then associate them with remotes
- **Tunnels** — `-L` (local forward) or `-R` (remote forward) rules referencing a remote
- **Topology view** — larger remote groups, direct click selection, and tunnel actions from the topology canvas
- **YAML config** — all config driven by `config.yaml` in the data root (see [Configuration](#configuration))
- **API token auth** — generated on first run; injected into the web UI automatically
- **Non-interactive SSH** — service-managed `known_hosts` trust store with configurable host-key policy
- **Equivalent SSH command preview** — inspect the concrete ssh command the service will launch for each tunnel
- **Daemon management** — install/start/stop/uninstall as a system service (launchd, systemd, SCM)
- **PWA** — installable as a progressive web app from the browser

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap HobaiRiku/tap
brew install ssh-tunnel-service
```

> **Upgrading:** `brew upgrade ssh-tunnel-service` only replaces the binary on
> disk — an already-installed service keeps running the **old** executable until
> it is restarted. After upgrading, restart it so the new binary takes effect:
>
> ```bash
> ssh-tunnel stop
> ssh-tunnel start
> ```
>
> `stop` shuts the running tunnels down gracefully (each `ssh` child is asked to
> exit and release its forwarded port) before the process exits, so the restart
> starts from a clean slate.

### Linux / macOS binaries

1. Open the [Releases](https://github.com/HobaiRiku/ssh-tunnel-service/releases) page.
2. Download the archive matching your platform, for example `ssh-tunnel-service_*_linux_amd64.tar.gz`.
3. Extract it and move `ssh-tunnel` onto your `$PATH`.

```bash
tar -xzf ssh-tunnel-service_*_linux_amd64.tar.gz
sudo install -m 0755 ssh-tunnel /usr/local/bin/ssh-tunnel
```

### Windows binaries

1. Download the matching `ssh-tunnel-service_*_windows_amd64.zip` release artifact.
2. Extract `ssh-tunnel.exe`.
3. Put it in a directory on your `PATH`, or run it directly from PowerShell.

```powershell
Expand-Archive .\ssh-tunnel-service_*_windows_amd64.zip -DestinationPath .\ssh-tunnel-service
$env:Path += ';' + (Resolve-Path .\ssh-tunnel-service)
ssh-tunnel.exe version
```

### `go install`

`go install` is intentionally **not supported**. The module path is `ssh-tunnel-service` rather than a `github.com/...` import path so releases are consumed from Homebrew or prebuilt binaries instead.

Likewise, **do not run `install` from `go run .`**. launchd/systemd/SCM need a stable executable path, so install the service from a release binary or a built binary already placed on your `PATH`.

## Quick start

```bash
# Run in foreground (development)
go run . run

# Or with make
make dev
```

Open: `http://localhost:2222`

On first run the service generates an API token and writes it to `<data-root>/token` (printed to stderr). The web UI retrieves the token automatically; CLI commands discover the running service and obtain the token over the loopback-only bootstrap endpoint, so they work even against the root-owned system service without direct access to its files.

## CLI reference

Resources are addressed by their unique `name`. `update` takes the current name
as its argument and accepts `--name` to rename (e.g. `tunnel update old --name new`).

The `remote`, `key`, and `tunnel` subcommands operate against the **running
service** over its local API (so the CLI and Web UI always agree on live state
and never edit `config.yaml` behind the service's back). The CLI finds the
service automatically — a foreground `ssh-tunnel run` instance you started is
preferred over the installed system service; pass `--home <dir>` (or set
`SSH_TUNNEL_HOME`) to target a specific instance. If no service is
running they exit with a hint to start it first (`ssh-tunnel start`, or
`ssh-tunnel run` in the foreground). Service-control commands (`install`,
`start`, `stop`, `status`, `tail`, `config`) work without it.

`install`, `uninstall`, `start`, and `stop` manage a **system-level** service and
require administrator privileges. Run them directly and the CLI will prompt for
elevation as needed — `sudo` on Linux/macOS, or a UAC dialog on Windows — so the
service starts at boot without an interactive login. The binary is copied to a
stable system location during `install`, so the installed service no longer
depends on where you ran the command from.

```text
ssh-tunnel [command]

Commands:
  run         Run the service in the foreground
  install     Install as a system service
  uninstall   Remove the system service registration
  start       Start the installed service
  stop        Stop the installed service
  status      Show service status
  tail        Tail the current service log in real time

  remote      Manage remote SSH targets
    list      List remotes (--json for machine-readable output)
    add       Add a remote
    update    Update a remote field and restart related running tunnels
    rm        Remove a remote

  key         Manage managed SSH private keys
    list      List managed keys (--json for machine-readable output)
    add       Add a key from pasted content or an existing file
    update    Update key metadata or replace stored key material
    rm        Remove a key

  tunnel      Manage SSH tunnel definitions
    list      List tunnels and live state (state + pid from the running service)
    add       Add a tunnel
    update    Update a tunnel field (running tunnels are restarted automatically)
    rm        Remove a tunnel
    start     Start a tunnel via the running service
    stop      Stop a running tunnel
    restart   Restart a tunnel via the running service

  config      Manage configuration
  version     Print version information
```

## Building

```bash
# Backend only
go build -o ssh-tunnel .

# Full release binary with embedded SPA
make build

# Cross-platform distribution matrix
make dist
```

## Web UI development

```bash
make ui-dev
make ui-build
```

## Configuration

The **data root** holds `config.yaml` and is resolved as: `--home` flag →
`SSH_TUNNEL_HOME` → platform default. The platform default depends on whether the
process is privileged, so the system service is independent of whoever installed
it:

| Platform | System service (elevated) | Per-user (`ssh-tunnel run`) |
| -------- | ------------------------- | --------------------------- |
| Linux | `/etc/ssh-tunnel-service` | `~/.ssh-tunnel-service` |
| macOS | `/Library/Application Support/ssh-tunnel-service` | `~/.ssh-tunnel-service` |
| Windows | `%ProgramData%\ssh-tunnel-service` | `~\.ssh-tunnel-service` |

The data root stores `config.yaml` plus:

- `token` — API token
- `known_hosts` — managed host key trust store
- `keys/` — uploaded or pasted private keys
- `logs/ssh-tunnel-service.log` — service log file

Every key, remote and tunnel is identified by its unique **`name`** — there is
no separate `id`. Remotes reference a key by name (`key:`), and tunnels
reference their remote by name (`remote:`). Renaming a remote or key cascades to
everything that references it (and restarts affected running tunnels).

> Migrating from an older config? Legacy `id` / `remote_id` / `key_id` fields are
> read on load and rewritten to the name-keyed form automatically; blank or
> duplicate names fall back to the old id and are de-duplicated.

Example managed key + remote association:

```yaml
keys:
  - name: deploy-key
    file: deploy-key

remotes:
  - name: Production
    host: ssh.example.com
    port: 22
    user: ubuntu
    key: deploy-key
```

If a remote does **not** set `key`, the service keeps using the normal system SSH behaviour (agent, default identity files, and ssh config resolution).

### Auto-start

A tunnel with `auto_start: true` is **supervised**: it starts immediately when
added or enabled, starts with the service, and is automatically reconnected
(with exponential backoff) if the underlying `ssh` process exits unexpectedly.
Stopping a tunnel cancels supervision until it is started again.

## Release automation

Tags matching `v*` trigger `.github/workflows/release.yml`, which runs GoReleaser to:

- build macOS / Linux / Windows archives
- attach checksums to the GitHub release
- publish/update the Homebrew formula in `HobaiRiku/homebrew-tap`

The workflow expects `HOMEBREW_TAP_GITHUB_TOKEN` to be configured in repository secrets.
