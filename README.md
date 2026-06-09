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
preferred over the installed system service. If no service is running they exit
with a hint to start it first (`ssh-tunnel start`, or `ssh-tunnel run` in the
foreground). `status` and `tail` also talk to the running service over the API
(read-only, no elevation): `tail` streams the log over a WebSocket, so it never
needs filesystem access to the data root. The remaining service-control commands
(`install`, `start`, `stop`, `config`) work without a running service.

### Choosing an instance (`connect`)

The system service, your per-user `ssh-tunnel run`, and any custom `--home`
instance can all run at once. `connect` sets the **persistent instance context**
the CLI attaches to by default:

```bash
ssh-tunnel connect system          # attach to the system service
ssh-tunnel connect user            # attach to your per-user instance
ssh-tunnel connect /path/to/home   # attach to a custom instance by data root
ssh-tunnel connect                 # interactively pick system / user / last custom
ssh-tunnel connect --show          # print the active context and its health
ssh-tunnel connect --clear         # reset to automatic discovery
```

The context is stored in `~/.ssh-tunnel-service/context.json` (only the active
selection plus the most recent custom home — no full history). `connect`
validates the target is reachable before saving it, and if a saved context later
becomes unreachable the CLI falls back to auto-discovery with a notice.

Attach precedence (highest first):

```text
--home flag  →  SSH_TUNNEL_HOME env  →  saved context  →  auto-discovery (user > system)
```

`--home` / `SSH_TUNNEL_HOME` remain **one-shot** overrides — they target a
specific instance for that invocation without changing the saved context. Every
resource command prints a `[ssh-tunnel @ <scope> · <address>]` banner to stderr
so you always know which instance you are operating on (the web UI shows the
same identity as a header badge). `status` reports the attached instance's
running state, PID, version, and uptime read-only over the API — no elevation
required.

### System vs. user install

By default `install` registers a **system-level** service (systemd system unit,
macOS LaunchDaemon, Windows SCM service) that starts at boot without an
interactive login. `install`, `uninstall`, `start`, and `stop` then require
administrator privileges — run them directly and the CLI prompts for elevation
(`sudo` on Linux/macOS, a UAC dialog on Windows).

```bash
ssh-tunnel install            # system service (prompts for elevation)
ssh-tunnel install --user     # per-user service (no root)
```

`install --user` instead registers a **per-user** service (systemd `--user`
unit, macOS LaunchAgent) that runs as you, needs no root, and uses your normal
ssh identities (agent / `~/.ssh`) exactly like `ssh-tunnel run`. Pair the
`--user` flag with `uninstall` / `start` / `stop` to manage it. After install the
CLI prints where it landed and how to manage it; on Linux, run
`sudo loginctl enable-linger "$USER"` if you want it to start before you log in.
**User-level services are not supported on Windows** (the SCM has no per-user
concept) — use a system install or `ssh-tunnel run`.

During `install` the running executable is copied to a stable location so the
service no longer depends on where you ran the command from. The copied file is
named **`ssh-tunnel`** (the CLI name) — `/usr/local/bin/ssh-tunnel` for a system
install, `~/.local/bin/ssh-tunnel` for `--user` (and
`%ProgramData%\ssh-tunnel-service\bin\ssh-tunnel.exe` on Windows). The OS service
itself is registered as `ssh-tunnel-service`.

### Keys and the system default

Because the system service runs without your login session (no `ssh-agent`,
no `~/.ssh`, no Keychain), it authenticates only with **managed keys** stored in
its own key store. `install` offers to import private keys from your `~/.ssh`,
and always generates a managed **system default key** (ed25519). Tunnels whose
remote binds no explicit key use this default under the system service; you can
rotate it (`key update`) or point the default at another key
(`key set-default`), but it cannot be deleted while designated. A per-user
install (or `ssh-tunnel run`) instead falls back to your normal ssh identities
for unbound tunnels, exactly like running `ssh` yourself.

Every managed key (imported or generated) stores its **public key** alongside
the private one, so you can install it on a target server. Print it with
`ssh-tunnel key pub <name>` (or copy it from the web UI / `key list --json`) and
append it to that server's `~/.ssh/authorized_keys`. Private keys are stored
`0600` inside the `0700` `keys/` directory.

The "equivalent ssh command" preview deliberately **omits** the managed key, so
you can copy it into your own session to verify connectivity with your normal
ssh identities.

```text
ssh-tunnel [command]

Commands:
  run         Run the service in the foreground
  install     Install as a service (--user for a per-user service)
  uninstall   Remove the service registration (--user)
  start       Start the installed service (--user)
  stop        Stop the installed service (--user)
  status      Show the attached instance's status (--json supported)
  tail        Stream the attached instance's log over the API (WebSocket)
  connect     Choose which instance the CLI attaches to (--show / --clear)

  remote      Manage remote SSH targets
    list      List remotes (--json for machine-readable output)
    add       Add a remote
    update    Update a remote field and restart related running tunnels
    rm        Remove a remote

  key         Manage managed SSH private keys
    list        List managed keys (--json includes the public key)
    add         Add a key from pasted content or an existing file
    update      Update key metadata or replace stored key material
    rm          Remove a key (the system default key cannot be removed)
    set-default Designate a key as the system default for unbound tunnels
    pub         Print a key's public key (authorized_keys line)

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

### Logging

The service writes a JSON log to `logs/ssh-tunnel-service.log` in the data root.
The **log level is fixed at `info`** and is not configurable. The file is rotated
automatically; the rotation and retention behaviour is configurable under `app:`:

```yaml
app:
  log_console: false      # also mirror logs to stderr (foreground runs)
  log_max_size_mb: 20     # rotate the log once it reaches this size (MB)
  log_max_backups: 10     # number of rotated files to keep
  log_max_age_days: 14    # days to preserve rotated files before deleting them
  log_compress: false     # gzip rotated files
```

Any field left unset (or `0`) falls back to the default shown above. Tail the
live log from any machine that can reach the service with `ssh-tunnel tail`.

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
