# Design: Instance context & the `connect` command

Status: accepted, in progress
Supersedes: the implicit "auto-discover only" CLI attach model.

## Background

The service already runs in three flavours, each fully isolated by its data
root (home):

- **system** — installed, elevated service; home at the platform system path
  (`/etc/ssh-tunnel-service`, `/Library/Application Support/...`,
  `%ProgramData%\...`).
- **user** — a foreground `ssh-tunnel run` started by an unprivileged user;
  home at `~/.ssh-tunnel-service`.
- **custom** — any instance pinned to an explicit `--home` / `SSH_TUNNEL_HOME`.

A running instance advertises its API address through a home-independent
runtime endpoint file (`internal/endpoint`), and the CLI is a thin API client:
it discovers an instance, fetches the token from the loopback-only
`/api/bootstrap`, and drives everything over HTTP. The web UI is served from
the same port and authenticates with the injected token.

Today the CLI's attach logic is purely automatic: user instance preferred over
system, with `--home`/env as an explicit override. When several instances run
at once this cannot express user intent, and there is no persistent notion of
"which instance am I working with".

## Goals

1. Add an explicit, persistent **instance context** the CLI attaches to by
   default — like `kubectl config use-context`.
2. Keep automatic discovery as the zero-config default.
3. Make the active instance **visible** in every CLI command and in the web UI,
   so the user always knows what they are operating on.
4. Drop the OS-privileged `status` path in favour of API/PID-based liveness.
5. First-class `--json` output across resource commands.

## Non-goals / explicitly rejected as over-design

- A full **history list** of every custom instance ever connected, plus
  management/cleanup commands. Custom instances are typically short-lived dev
  runs; tracking a full registry is high cost / low value. We keep **only the
  last custom home** instead.
- A dedicated `status` HTTP endpoint. Liveness is derived from the endpoint
  file + a `/api/health` probe; no new endpoint is needed.

## The context model

A user-level context file records which instance the CLI attaches to by
default. It lives in the **invoking user's** home, never in any instance's
home:

`~/.ssh-tunnel-service/context.json`

```json
{
  "scope": "custom",        // "auto" | "system" | "user" | "custom"
  "home":  "/path/to/home" // present only when scope == "custom"
}
```

- `auto` (default when no file): current behaviour — discover user, then system.
- `system` / `user`: pin to that scope's discovered endpoint.
- `custom`: pin to the instance at `home`.

### Attach precedence

Highest wins:

```
--home flag
  → SSH_TUNNEL_HOME env
    → context.json (scope != "auto")
      → auto-discovery (user > system)
```

`--home` and `SSH_TUNNEL_HOME` are *one-shot* overrides; they do not mutate the
persisted context. Only `connect` writes the context file.

### Health & fallback

`connect` validates the target before persisting: it must have a reachable
endpoint and pass `/api/bootstrap` (which doubles as liveness). Unreachable
targets produce a clear error and the context is **not** changed.

At normal command time, if the persisted context points at an instance that is
no longer healthy (e.g. a custom dev run that has exited), the CLI does **not**
hard-fail. It falls back to auto-discovery and prints a one-line notice to
stderr:

```
ssh-tunnel: last connected instance (custom /path) is not reachable; falling back to auto-discovery
```

## The `connect` command

```
connect [system | user | <home-path>]   # switch the active context
connect                                  # interactive pick (system / user / last-custom)
connect --show                           # print the active context + resolved address + health
connect --clear                          # reset to auto-discovery
```

- `connect system` / `connect user` — pin to that scope.
- `connect <path>` — pin to a custom home; the path is remembered as
  `last-custom`.
- `connect` with no args and a TTY — list `system`, `user`, and `last-custom`
  (if any), let the user choose. Non-TTY with no args is an error (nothing to
  pick).
- `connect --show` — never mutates; prints `scope`, resolved address, home, and
  reachable/unreachable.
- `connect --clear` — delete the persisted scope (back to `auto`); keeps the
  remembered `last-custom` so a bare `connect` can still offer it.

Only the *active* selection plus a single `last-custom` slot are persisted — no
broader history, by design.

## Instance visibility

Every resource command prints a one-line banner to **stderr** before its
output, so it never pollutes stdout / JSON:

```
[ssh-tunnel @ system · 127.0.0.1:2222]
```

Format: `[ssh-tunnel @ <scope>[ <home-tail>] · <address>]`, where `<home-tail>`
is the last path segment for custom instances. Suppressed when `--json` is set
on a machine-readable command? No — the banner is on stderr, JSON is on stdout,
so both coexist; scripts reading stdout are unaffected.

The web UI shows the same identity (scope + home tail + address) in a header
badge, sourced from `/api/version` (extended) or a small `/api/instance`
payload.

## `status` rework

`status` no longer shells out to the OS service manager (which can need
elevation). Instead it:

1. Resolves the target via the same attach precedence.
2. Reports running/stopped from the endpoint file + `/api/health` probe.
3. Prints PID/address/home/uptime/version from the discovery record + API.

This makes `status` a read-only, unprivileged operation consistent with the
rest of the CLI.

## `--json` output

Resource commands (`remote list`, `key list`, `tunnel list`, plus `status` and
`connect --show`) gain a stable `--json` flag. The existing list JSON shapes are
preserved (documented "AI-ready CLI" contract); new commands follow the same
convention.

## Security note: cross-user endpoint visibility

For a normal user to attach to the **system** instance, two things must hold,
both already true by design:

1. The system runtime dir (`/run/ssh-tunnel-service` on Linux) and the endpoint
   file are world-readable (`0755` dir, `0644` file), so any local user can
   read the advertised port.
2. `/api/bootstrap` is loopback-only and unauthenticated, returning the token to
   any local client.

This is the intended model: local users may drive the root-owned service
read/write over loopback. (If finer-grained access control is ever wanted, it
belongs at the API auth layer, not the endpoint file.) The implementation must
ensure the system runtime dir is created `0755`, not `0700`.

## Rollout / commits

1. `endpoint`: confirm/repair `0755` system runtime dir; add a `Probe`/health
   helper and a `Lookup(scope)` accessor.
2. `cmd`: context store (`~/.ssh-tunnel-service/context.json`), attach
   precedence in `newAPIClient`, fallback notice.
3. `cmd`: `connect` command (switch / interactive / `--show` / `--clear`).
4. `cmd`: instance banner on resource commands; `--json` coverage.
5. `status` rework over API/PID; drop OS-privileged path.
6. web UI: instance identity badge (+ `/api/instance` if needed).
7. docs: README CLI section, this design doc.
```