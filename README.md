# ssh-tunnel-service

Cross-platform Go daemon that manages SSH `-L` / `-R` port-forwarding tunnels via a CLI, REST API, and an embedded Vue 3 web UI with interactive topology, managed SSH keys, and English/中文 UI switching.

## Features

- **Remotes** — reusable SSH server definitions (host, port, user, optional managed key)
- **Keys** — paste or upload private keys into the runtime config directory, then associate them with remotes
- **Tunnels** — `-L` (local forward) or `-R` (remote forward) rules referencing a remote
- **Topology view** — larger remote groups, direct click selection, and tunnel actions from the topology canvas
- **YAML config** — all config driven by `~/.ssh-tunnel-service/config.yaml`
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

On first run the service generates an API token and writes it to `~/.ssh-tunnel-service/token` (printed to stderr). The web UI retrieves the token automatically; CLI commands also read it from the same file.

## CLI reference

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
    list      List tunnels and live state
    add       Add a tunnel
    update    Update a tunnel field (running tunnels are restarted automatically)
    rm        Remove a tunnel
    start     Start a tunnel via the running service
    stop      Stop a running tunnel

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

Default location: `~/.ssh-tunnel-service/config.yaml`

The runtime directory also stores:

- `token` — API token
- `known_hosts` — managed host key trust store
- `keys/` — uploaded or pasted private keys
- `logs/ssh-tunnel-service.log` — service log file

Example managed key + remote association:

```yaml
keys:
  - id: deploy-key
    name: Deploy key
    file: deploy-key

remotes:
  - id: prod
    name: Production
    host: ssh.example.com
    port: 22
    user: ubuntu
    key_id: deploy-key
```

If a remote does **not** set `key_id`, the service keeps using the normal system SSH behaviour (agent, default identity files, and ssh config resolution).

## Release automation

Tags matching `v*` trigger `.github/workflows/release.yml`, which runs GoReleaser to:

- build macOS / Linux / Windows archives
- attach checksums to the GitHub release
- publish/update the Homebrew formula in `HobaiRiku/homebrew-tap`

The workflow expects `HOMEBREW_TAP_GITHUB_TOKEN` to be configured in repository secrets.
