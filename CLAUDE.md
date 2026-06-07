# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Cross-platform Go daemon that manages SSH `-L` / `-R` port forwards, exposing the same operations through a Cobra CLI, a token-authenticated REST API, and an embedded Vue 3 SPA. See `README.md` for end-user docs.

## Common commands

The `Makefile` is the source of truth — prefer it over raw `go`/`pnpm` invocations.

Backend:

```bash
make run        # foreground dev run; sets SSH_TUNNEL_HOME=$PWD/.ssh-tunnel-service-home
make build      # release binary; runs ui-build then `go build -tags embedui` into build/bin/
make test       # `go test ./...` with the local home
make fmt vet tidy
make dist       # cross-compile to build/dist/ + SHA256SUMS
```

A single Go test: `go test ./internal/services -run TestManager_StartStop -v` (the project-wide local home is set automatically by `make test`; for direct `go test` set `SSH_TUNNEL_HOME=$PWD/.ssh-tunnel-service-home`).

UI (in `ui/`, pnpm workspace; prefix with `corepack` if pnpm isn't global):

```bash
make ui-dev     # Vite dev server on :5173, proxies /api to :2222
make ui-build   # compiles SPA into internal/web/static/ (consumed by embedui tag)
cd ui && pnpm type-check   # vue-tsc
cd ui && pnpm lint
```

The bare `go build .` produces a binary **without** the SPA — `internal/web/embed.go` then serves a "UI not built" placeholder page while `/api/*` remains fully functional. Embedding only happens under the `embedui` build tag (`internal/web/embed_ui.go`), which `make build` / `make dist` apply.

## Architecture

### Composition root

`main.go` decides between interactive CLI (`cmd.Execute`) and OS-managed service mode (`service.RunService`) via `kardianos/service`. Both paths funnel through `internal/service.Run` → `internal/app.Run`, which is the single place that wires:

```
paths.Resolve → config.LoadWithDefaults → log.Init → services.{Runtime,Registry,Manager} → api.NewRouter → web.Mount → http.Server
```

When adding cross-cutting state, plug it in there rather than reaching into subpackages from the CLI.

### `internal/services` is the only place that mutates state

Both the HTTP API (`internal/api/router.go`) and CLI subcommands in `cmd/` must call through `services.Registry` / `services.Manager`. The registry holds the config under a `sync.RWMutex`, clones it on every write, validates, then persists atomically via `config.Write` — never edit `config.yaml` from anywhere else, or CLI and Web UI will drift.

Every key, remote and tunnel is keyed by its unique `name` (there is no `id`). Remotes reference a key via `Remote.Key` (a key name); tunnels reference their remote via `Tunnel.Remote` (a remote name). `config.ValidateName` enforces the name rules and `internal/config/migrate.go` rewrites legacy `id`/`remote_id`/`key_id` configs to the name-keyed form on load.

- `Registry` — CRUD over remotes/tunnels, owns persistence; renaming a remote/key cascades to referencing resources
- `Runtime` — in-memory `tunnelName → {state, pid, error}` map, the only source of truth for live state
- `Manager` — owns `ssh` child processes (keyed by tunnel name); on `Start` it builds args, on `Wait` it translates stderr into a human diagnostic via `diagnoseSSHFailure` and writes the result back into `Runtime`. `auto_start` tunnels are supervised: `Manager` reconnects them with backoff on unexpected exits until they are stopped (`Stop`) or removed (`Forget`).

`TunnelStatus` (returned to API/CLI) is always `config.Tunnel ⊕ Runtime.Get(name)` — read it from `Registry.ListTunnels` / `GetTunnel`, never compose it ad-hoc.

### SSH is invoked non-interactively, on purpose

`Manager.Start` always passes `BatchMode=yes`, `PasswordAuthentication=no`, `KbdInteractiveAuthentication=no`, `NumberOfPasswordPrompts=0`. A remote that only accepts a password **must** fail fast and surface a diagnostic — do not add interactive fallbacks. Host-key behavior is selected by `app.ssh_host_key_policy` (`accept-new` default, `strict`, `insecure`) and a service-managed `known_hosts` lives under `<SSH_TUNNEL_HOME>/known_hosts`, never `~/.ssh/known_hosts`. `sshHostKeyArgs` is the only mapping; new policies belong there.

Key injection is **scope-dependent** (`Manager.systemService`, set from `elevate.IsElevated()` via `app.Options.SystemService`). A tunnel whose remote binds an explicit key always uses it. For an *unbound* remote: a **system service** injects the managed default key (`app.system_default_key`), since it has no login session / agent; a per-user (session) run injects **no** `-i` and lets ssh pick its own identities. `Registry.EnsureSystemDefaultKey` (called from `app.Run` on system start, and during install provisioning) generates an ed25519 key and designates it; `DeleteKey` refuses to remove the currently designated default (switch via `SetSystemDefaultKey` / `key set-default` first). `Manager.Command` (the equivalent-command preview) deliberately omits the key — it is a session-equivalent command, not the exact non-interactive invocation.

### Paths & home dir

`internal/paths` resolves the data root in this precedence: `--home` flag → `SSH_TUNNEL_HOME` env → **platform default**. The platform default depends on privilege (`elevate.IsElevated()`) so the system service is decoupled from whoever ran `install`: elevated it is a system path (Linux `/etc/ssh-tunnel-service`, macOS `/Library/Application Support/ssh-tunnel-service`, Windows `%ProgramData%\ssh-tunnel-service`), otherwise the per-user `$HOME/.ssh-tunnel-service`. The platform split lives in `home_{linux,darwin,windows,other}.go`. Always go through `paths.Paths` helpers (`Config()`, `Token()`, `KnownHosts()`, `LogFile()`) rather than concatenating strings. Dev runs use `$PWD/.ssh-tunnel-service-home` (set by `make run`/`make test`).

### Auth & UI bootstrap

The API token lives in its own file (`<home>/token`), separate from `config.yaml`, generated on first start. `internal/api/router.go` requires `Authorization: Bearer <token>` for everything except `/api/health` and `/api/bootstrap`. `/api/bootstrap` is **loopback-only** and returns the token so the SPA can authenticate. When embedded, `internal/web/embed.go` additionally injects `window.__AUTH_TOKEN__` into `index.html` before serving — the SPA prefers the injection and falls back to `/api/bootstrap` (`ui/src/api/client.ts`). The PWA config in `ui/vite.config.ts` deliberately excludes `index.html` from precache so the injected token never goes stale.

### CLI

Cobra tree in `cmd/`. All resource subcommands (`remote`/`key`/`tunnel`, list **and** mutations) go through the running service's REST API via the `apiClient` in `cmd/http.go`. `cmd/http.go`'s `resolveClient` is the single attach resolver, in precedence order: `--home` flag → `SSH_TUNNEL_HOME` env → **persisted context** (`connect`) → **auto-discovery** (`internal/endpoint`, user instance preferred over system). It obtains the token from the loopback-only `/api/bootstrap`, so a normal user can drive the root-owned system service without read access to its home. `--home`/`SSH_TUNNEL_HOME` are **one-shot** overrides targeting that specific instance directly (address from its `config.yaml`, token from its token file, falling back to bootstrap) and never mutate the context. This keeps the CLI from editing `config.yaml` behind a live service (which would drift from its in-memory state and be overwritten on the next persist) and means `tunnel list` shows the service's live `state`/`pid`. When no service is running, `apiClient.request` returns a "no service is running … start it with `ssh-tunnel start`" error. `status` and `tail` also go through the running service (read-only); only the service-control commands (`install`/`start`/`stop`/`connect`) and the `config` file commands work without a running service. Keep the JSON output formats for `remote list` / `key list` / `tunnel list` stable — the README documents them as the "AI-ready CLI" contract.

The `connect` command (`cmd/connect.go`) sets a **persistent instance context** stored in the invoking user's `~/.ssh-tunnel-service/context.json` (`cmd/context.go`): `scope` (`auto`/`system`/`user`/`custom`), the custom `home`, and a single `last_custom` slot — deliberately **not** a full history. `connect` validates the target via `/api/health` before persisting; `resolveClient` falls back to auto-discovery (with a stderr notice) when a pinned context is unreachable. Every resource command prints an instance banner (`[ssh-tunnel @ <scope> · <addr>]`) to **stderr** via `newAPIClient` (suppress with `bannerSuppressed` when a command formats its own identity, e.g. `status`, `connect --show`) — stdout/JSON stay clean. `status` is read-only over `/api/instance` (scope/home/pid/version/uptime), not the OS service manager, so it needs no elevation. `/api/instance` is also consumed by the web UI header badge (`ui/src/App.vue`). `tail` streams the service log over the `/api/logs/stream` WebSocket (`internal/api/logs.go`, gorilla/websocket): the backend tails `Paths.LogFile()` (last-N lines then follows appends) and the CLI (`apiClient.streamLogs`) dials it with the bearer token, so no filesystem access to the data root is needed.

The running instance advertises its address via `endpoint.Write` in `app.Run` (removed on shutdown); the file lives in a home-independent runtime dir keyed by scope (`/run/ssh-tunnel-service` for system, `$XDG_RUNTIME_DIR/...` for user on Linux — see `dir_{linux,darwin,windows,other}.go`). It carries only the address, never the token.

The service tears tunnels down cleanly on shutdown: `Manager.Shutdown` (called from `app.Run` once the HTTP server stops) marks every tunnel undesired, cancels reconnect timers, and gracefully `terminate`s each `ssh` child, blocking until they exit. `Manager.Start` also sets `cmd.Cancel` (SIGTERM) + `cmd.WaitDelay` so a cancelled `baseCtx` still releases forwarded ports instead of orphaning processes.

### Service install (`internal/service`)

Uses `kardianos/service` for launchd / systemd / SCM, in **two scopes** threaded as a `user bool` through `service.{Install,Uninstall,Start,Stop,New,newWithExe}` (and `BinaryPath`/`rollbackBinary`). System scope (default) registers a system-level service (systemd system unit, macOS **LaunchDaemon** in `/Library/LaunchDaemons`, Windows SCM service) that survives reboots without a login. User scope (`--user`, kardianos `Option["UserService"]=true`) registers a per-user service (systemd `--user` unit, macOS **LaunchAgent** in `~/Library/LaunchAgents`) that runs unprivileged and uses the user's own ssh identities (`SystemService=false`, no managed default key) — **unsupported on Windows** (`checkUserScope` in `userscope_{windows,other}.go` returns an error; `service.UserScopeSupported()` gates the CLI). The macOS `darwin*` wrappers in `launchctl_darwin.go` take the scope and drive `bootstrap`/`bootout`/`kickstart` in the **`system`** domain (system) or **`gui/<uid>`** domain (user); don't bypass them. `install` first copies the running binary to a stable path (`binary.go`/`bindest_*.go`): the file is named after the CLI (**`ssh-tunnel`**, not the service name `ssh-tunnel-service`) — `/usr/local/bin/ssh-tunnel` (system Unix), `~/.local/bin/ssh-tunnel` (user Unix), `%ProgramData%\ssh-tunnel-service\bin\ssh-tunnel.exe` (Windows). It pins the unit to it via `kservice.Config.Executable`, rolling the copy back (only when this run created it) if registration fails; `uninstall` removes it. The running daemon derives its scope from privilege (`RunService` → `!elevate.IsElevated()`), matching how the home dir resolves.

The **system** control commands (`install`/`uninstall`/`start`/`stop`) require admin/root; the matching `--user` variants run unprivileged (`install --user` is even rejected under sudo so it doesn't bake in root's paths). `cmd/ensurePrivileged` (backed by `internal/elevate`) re-executes the command elevated when it isn't already: `sudo` on Linux, `sudo` or the macOS authentication dialog (`osascript`) depending on TTY, and a Windows UAC prompt (`ShellExecute` `runas`). The platform split lives in `elevate_{linux,darwin,windows}.go`. `elevate.Ensure` takes extra args to forward to the elevated child — system `install` uses this to carry interactively-chosen `--import-key` paths across the privilege boundary (it scans `~/.ssh` *before* elevating so it reads the invoking user's keys, then `service.Provision` imports them into the system home as root and calls `EnsureSystemDefaultKey`). `--user` install skips the prompt/provision entirely.

Every managed key materializes a public key: `Registry.prepareKey` writes `<file>.pub` (via `config.PublicKeyAuthorized`) next to the `0600` private key, and `GetKey`/`ListKeys` populate the non-persisted `config.SSHKey.Public` (`yaml:"-"`) from it (deriving from the private key if the `.pub` is missing). Exposed via `key pub <name>`, `key list --json`, and the web UI "copy public key" action.

## Conventions

- Go: error-as-value via tuples; reserve `try/catch`-style wrapping (`fmt.Errorf("...: %w", err)`) for adding context at boundaries.
- Branching: prefer guard clauses and map dispatch (`sshHostKeyArgs`, `diagnoseSSHFailure`) over `else if` chains.
- JS/TS (in `ui/`): ES modules only. The package has `"type": "module"`; any standalone Node script outside the package needs `.mjs`, and any CJS-only file inside it needs `.cjs`. Prefer `async`/`await` over chained `.then`.
- Markdown: leave blank lines around fenced code blocks (DingTalk compatibility).

## Notes for future changes

- Adding a config field: extend `internal/config/schema.go`, update `internal/config/validate.go`, `example.go`, and ensure `config.LoadWithDefaults` fills a sensible default — `Registry.persist` will reject anything that fails `Validate`.
- Adding an API endpoint: register in `internal/api/router.go` inside the `api.Group("/api")` block so `tokenAuth` applies. Only `/api/health` and `/api/bootstrap` are unauthenticated.
- Adding a tunnel field that affects the spawned `ssh` command: thread it through `Manager.Start` arg construction; mirror it into CLI flags in `cmd/tunnel.go` and into the SPA's `Tunnel` type in `ui/src/api/client.ts`.
- *Deleting* a remote referenced by a tunnel is rejected by `Registry.DeleteRemote`; do not loosen that check. *Renaming* a remote/key is allowed and cascades to referencing resources (`Tunnel.Remote` / `Remote.Key`) — keep the cascade and the duplicate-name guard intact.
- Resources are addressed by `name` in API paths (`/api/tunnels/:name`, URL-encoded by the SPA), CLI args, and references. Forms validate names client-side via `ui/src/validation.ts` mirroring `config.ValidateName`.
