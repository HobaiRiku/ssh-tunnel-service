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

### Paths & home dir

`internal/paths` resolves the data root in this precedence: `--home` flag → `SSH_TUNNEL_HOME` env → `$HOME/.ssh-tunnel-service`. Always go through `paths.Paths` helpers (`Config()`, `Token()`, `KnownHosts()`, `LogFile()`) rather than concatenating strings. Dev runs use `$PWD/.ssh-tunnel-service-home` (set by `make run`/`make test`).

### Auth & UI bootstrap

The API token lives in its own file (`<home>/token`), separate from `config.yaml`, generated on first start. `internal/api/router.go` requires `Authorization: Bearer <token>` for everything except `/api/health` and `/api/bootstrap`. `/api/bootstrap` is **loopback-only** and returns the token so the SPA can authenticate. When embedded, `internal/web/embed.go` additionally injects `window.__AUTH_TOKEN__` into `index.html` before serving — the SPA prefers the injection and falls back to `/api/bootstrap` (`ui/src/api/client.ts`). The PWA config in `ui/vite.config.ts` deliberately excludes `index.html` from precache so the injected token never goes stale.

### CLI

Cobra tree in `cmd/`. All resource subcommands (`remote`/`key`/`tunnel`, list **and** mutations) go through the running service's REST API via the `apiClient` in `cmd/http.go`, which reads the listen address + token from the home dir. This keeps the CLI from editing `config.yaml` behind a live service (which would drift from its in-memory state and be overwritten on the next persist) and means `tunnel list` shows the service's live `state`/`pid`. When the service isn't running, `apiClient.request` returns a "no service is running … start it with `ssh-tunnel start`" error. Only the service-control commands (`install`/`start`/`stop`/`status`/`tail`) and the `config` file commands work without a running service. Keep the JSON output formats for `remote list` / `key list` / `tunnel list` stable — the README documents them as the "AI-ready CLI" contract.

The service tears tunnels down cleanly on shutdown: `Manager.Shutdown` (called from `app.Run` once the HTTP server stops) marks every tunnel undesired, cancels reconnect timers, and gracefully `terminate`s each `ssh` child, blocking until they exit. `Manager.Start` also sets `cmd.Cancel` (SIGTERM) + `cmd.WaitDelay` so a cancelled `baseCtx` still releases forwarded ports instead of orphaning processes.

### Service install (`internal/service`)

Uses `kardianos/service` for launchd / systemd / SCM. On macOS the service runs as a **user agent** (`UserService: true`); install/uninstall additionally call `darwinBootstrap`/`darwinBootout` (see `launchctl_darwin.go`) so it actually starts. Don't bypass those wrappers.

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
