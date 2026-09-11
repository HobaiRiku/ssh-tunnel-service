# ssh-tunnel command reference

Complete CLI surface. `ssh-tunnel <command> --help` is authoritative for the
installed build; this file is the contract to construct commands from.

## Instance selection

Resource commands talk to a **running** service over its loopback REST API, so
they never need elevation and never edit the config file behind the service's
back. The target instance is resolved in this order:

1. `--home <path>` flag — one-shot, targets that data root directly
2. `SSH_TUNNEL_HOME` environment variable — one-shot
3. persisted context set by `connect`
4. auto-discovery (a running user instance is preferred over the system one)

Every resource command prints `[ssh-tunnel @ <scope> · <addr>]` to **stderr**;
stdout stays clean, so `--json` output can be piped straight into a parser.

| Command | Purpose |
| --- | --- |
| `connect` | interactively pick system / user / last custom |
| `connect system` \| `connect user` \| `connect <home-path>` | pin the context |
| `connect --show` | print the active context and its health |
| `connect --clear` | reset to automatic discovery |

`connect` validates the target over `/api/health` before saving it. A pinned
context that later becomes unreachable falls back to auto-discovery with a
notice on stderr.

## status

```bash
ssh-tunnel status [--json]
```

Read-only, over the API — no elevation. JSON fields:

```json
{
  "running": true,
  "scope": "system",
  "address": "127.0.0.1:2222",
  "home": "/Library/Application Support/ssh-tunnel-service",
  "pid": 40384,
  "version": "0.0.9",
  "uptime_seconds": 3831.74
}
```

## remote

A remote is a reusable SSH target (`user@host:port`) that tunnels reference by
name.

| Command | Flags |
| --- | --- |
| `remote list [--json]` | — |
| `remote add` | `--name` (required), `--host` (required), `--port` (default 22), `--user`, `--key`, `--description` |
| `remote update <name>` | `--name` (rename), `--host`, `--port`, `--user`, `--key` (empty string clears), `--description` |
| `remote rm <name>` | — |

`--key` names a managed key from `key list`; leaving it empty means "unbound"
(see *Key resolution* below). Renaming a remote cascades to every tunnel that
references it. Removing a remote that a tunnel still references is rejected —
delete or repoint the tunnels first. Updating a field restarts the running
tunnels that depend on it.

`remote list --json`:

```json
[{ "name": "prod-db", "host": "10.0.0.5", "port": 22, "user": "deploy", "key": "deploy" }]
```

## key

Managed private keys live in the instance's own key store (`<home>/keys/`), not
in `~/.ssh`. Each one materialises a `.pub` alongside it.

| Command | Flags |
| --- | --- |
| `key list [--json]` | — |
| `key add` | `--name` (required), `--source <path>` (copy an existing private key file) or `--private-key <content>`, `--file-name` (stored file name override), `--description` |
| `key update <name>` | `--name` (rename), `--source`, `--private-key`, `--file-name`, `--description` |
| `key rm <name>` | — |
| `key pub <name>` | prints the `authorized_keys` line |
| `key set-default <name>` | designate the system default key for unbound tunnels |

`key rm` refuses to remove the key currently designated as the system default —
point `key set-default` elsewhere first. Renaming a key cascades to the remotes
that bind it.

`key list --json`:

```json
[{ "name": "deploy", "file": "deploy", "description": "Imported from ~/.ssh/id_ed25519", "public_key": "ssh-ed25519 AAAA… host" }]
```

The private key itself is never returned by the API or printed.

## tunnel

| Command | Flags |
| --- | --- |
| `tunnel list [--json]` | — |
| `tunnel add` | `--name` (required), `--remote` (required), `--direction` (`-L` default, or `-R`), `--bind-addr` (default `127.0.0.1`), `--bind-port` (required), `--target-host` (required), `--target-port` (required), `--auto-start`, `--description` |
| `tunnel update <name>` | `--name` (rename), `--remote`, `--direction`, `--bind-addr`, `--bind-port`, `--target-host`, `--target-port`, `--auto-start`, `--description` |
| `tunnel rm <name>` | — |
| `tunnel start <name>` | — |
| `tunnel stop <name>` | — |
| `tunnel restart <name>` | stop if running, then start |

`tunnel add` only defines the tunnel; it does not start it. `tunnel update` on a
running tunnel restarts it so the change takes effect.

Reach a remote-only Postgres at `localhost:5432` on the server, as `localhost:15432` here:

```bash
ssh-tunnel tunnel add --name prod-pg --remote prod-db --direction -L \
  --bind-addr 127.0.0.1 --bind-port 15432 \
  --target-host 127.0.0.1 --target-port 5432 --auto-start
ssh-tunnel tunnel start prod-pg
```

Publish the local dev server on port 3000 as port 8080 on the remote:

```bash
ssh-tunnel tunnel add --name dev-preview --remote prod-db --direction -R \
  --bind-addr 0.0.0.0 --bind-port 8080 \
  --target-host 127.0.0.1 --target-port 3000 --auto-start
ssh-tunnel tunnel start dev-preview
```

Move a tunnel to a different local port, then confirm:

```bash
ssh-tunnel tunnel update prod-pg --bind-port 15433
ssh-tunnel tunnel list --json
```

`tunnel list --json` returns the definition plus live runtime state:

```json
[{
  "name": "prod-pg",
  "remote": "prod-db",
  "direction": "-L",
  "bind_address": "127.0.0.1",
  "bind_port": 15432,
  "target_host": "127.0.0.1",
  "target_port": 5432,
  "auto_start": true,
  "state": "running",
  "pid": 53293
}]
```

`state` is one of `running`, `stopped`, `error`. Empty optional fields
(`description`, `ssh_options`, `pid`, `error`) are omitted rather than sent as
empty values, so read them defensively. `error` is already a plain-language
diagnosis, not raw ssh stderr:

```json
{ "name": "prod-pg", "remote": "prod-db", "state": "error",
  "error": "ssh connection refused; verify the remote host, port, and sshd availability" }
```

`ssh_options` (extra `-o` flags per tunnel) exists in the config and API but has
no `tunnel add`/`update` flag — set it from the web UI or the REST API. It is
rarely needed: keepalives are already applied to every tunnel.

## Forward semantics

Both directions build the ssh spec `bind_address:bind_port:target_host:target_port`.

- `-L` — the **service machine** listens on `bind_address:bind_port`; connections
  are forwarded and `target_host:target_port` is resolved **from the remote
  server**.
- `-R` — the **remote server** listens on `bind_address:bind_port`; connections
  come back and `target_host:target_port` is resolved **from the service
  machine**. A non-loopback `bind_address` requires `GatewayPorts yes` in the
  remote sshd config.

## How the ssh process is built

Every tunnel runs `ssh -N` with these fixed options, in this order:

```
-o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no
-o NumberOfPasswordPrompts=0 -o ExitOnForwardFailure=yes
-o ServerAliveInterval=15 -o ServerAliveCountMax=3 -o TCPKeepAlive=yes
<host key policy options> [-i <key> -o IdentitiesOnly=yes]
<direction> <forward spec> <tunnel ssh_options…> -p <port> <user>@<host>
```

Consequences worth knowing:

- Password and keyboard-interactive auth are impossible by design. A remote that
  only accepts a password fails fast with a diagnostic; do not look for an
  interactive fallback.
- `ExitOnForwardFailure=yes` means a bind port already in use is a hard start
  failure, not a silent half-open tunnel.
- Keepalives are already configured; per-tunnel `ServerAliveInterval` options
  are redundant.
- `auto_start` tunnels are supervised: unexpected exits are retried with backoff
  until the tunnel is stopped or removed.

Host key policy comes from `app.ssh_host_key_policy`:

| Policy | Behaviour |
| --- | --- |
| `accept-new` (default) | trust an unknown host on first contact, refuse a changed one |
| `strict` | only hosts already in the managed `known_hosts` |
| `insecure` | no host key checking (`UserKnownHostsFile=/dev/null`) |

The managed `known_hosts` lives under the instance home, never `~/.ssh/known_hosts`.

## Key resolution

| Situation | Identity used |
| --- | --- |
| remote binds a key (`--key`) | that key, with `IdentitiesOnly=yes` |
| unbound remote, **system** service | the managed default key (`app.system_default_key`) |
| unbound remote, **user** service | no `-i`; ssh picks its own identities (`~/.ssh`, agent) |

The system service has no login session or agent, which is why it needs a
managed key. Moving tunnels from a user-scope instance to a system-scope one
therefore changes which key authenticates — bind the remotes to an imported key,
or authorize the system default key on the servers.

## Service lifecycle

These act on the OS service manager, not the API.

| Command | Scope | Privilege |
| --- | --- | --- |
| `install [--import-key <path>…]` | system service (boot start) | root; elevates automatically |
| `install --user` | per-user service | no root; rejected under sudo |
| `uninstall [--user]` | matching scope | root for system |
| `start [--user]` / `stop [--user]` | matching scope | root for system |
| `run` | foreground, no service manager | — |

`--import-key` is repeatable and copies private keys from the invoking user's
`~/.ssh` into the system key store (read before elevating, so root's home is not
used by mistake). A managed default key is always generated on system install.
`--user` is unsupported on Windows.

## Logs

```bash
ssh-tunnel tail [-n <lines>] [-f]
```

Streams the service log over the API (WebSocket): `-n` is how many recent lines
to print first (default 50), `-f` follows appends (default true). No filesystem
access to the data root is needed, so it works against the root-owned system
service as a normal user.

## Config

```bash
ssh-tunnel config show               # effective configuration as YAML
ssh-tunnel config path               # config file path
ssh-tunnel config edit               # open in $EDITOR
ssh-tunnel config known-hosts-path   # effective managed known_hosts path
```

These read the file directly rather than going through the API, so they can need
read access to the instance home. Do not use `config edit` to change remotes,
keys or tunnels while the service runs — the service owns that state in memory
and will overwrite the file.

App-level fields:

| Field | Meaning |
| --- | --- |
| `http_listen` | API/UI listen address (default `127.0.0.1:2222`) |
| `log_level`, `log_console` | logging verbosity and console mirroring |
| `log_max_size_mb`, `log_max_backups`, `log_max_age_days`, `log_compress` | rotation |
| `ssh_host_key_policy` | `accept-new` \| `strict` \| `insecure` |
| `ssh_known_hosts_file` | absolute path to the managed known_hosts |
| `system_default_key` | key name injected for unbound remotes under the system service |

## Validation rules

- Names: non-empty, valid UTF-8, at most 64 characters, no leading or trailing
  whitespace, no `/` or `\`, no control characters. Non-ASCII names are allowed.
- Names are unique per kind, and are the reference between resources
  (`tunnel.remote` → remote name, `remote.key` → key name).
- Ports: 1–65535 for `port`, `bind_port`, `target_port`.
- `direction` must be exactly `-L` or `-R`.
- `host`, `user` and `target_host` are required and non-empty.

## Skill maintenance

```bash
ssh-tunnel skill install                       # into every detected client
ssh-tunnel skill install --target claude,codex # regardless of detection
ssh-tunnel skill install --project             # into the current repository
ssh-tunnel skill install --dir <path> [--plugin]
ssh-tunnel skill install --agents-md           # inject a block into the repo AGENTS.md
ssh-tunnel skill uninstall
ssh-tunnel skill print [--reference]
```

Installs are idempotent and report `installed` / `updated` / `up to date`. A
destination this tool did not write, or one edited by hand, is refused unless
`--force`; `--dry-run` shows the plan. Re-run after upgrading the binary so the
skill matches the CLI it documents. Never run `skill install` under `sudo` — it
would install into root's home.
