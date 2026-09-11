---
name: ssh-tunnel
description: >
  Manage long-lived SSH port forwards through the ssh-tunnel service CLI. Use
  this skill whenever the user wants to forward, expose, publish or tunnel a
  port over SSH — reaching a database or service that only listens on a remote
  host, exposing a local dev server on a jump box — or wants to inspect, start,
  stop or debug forwards that already exist. Triggers on "ssh tunnel", "port
  forward", "-L", "-R", "expose my local port on the server", "reach the
  remote database locally", "why is my tunnel down".
---

# ssh-tunnel

`ssh-tunnel` drives a background service that owns long-lived `ssh -L` / `ssh -R`
port forwards: it supervises them, reconnects them with backoff when they drop,
and brings them back at boot. Prefer it over spawning a raw `ssh -L` in a shell,
which dies with the shell and nobody restarts.

## Rules

1. **Drive everything through the CLI.** Never hand-edit `config.yaml`. The
   running service holds the config in memory and rewrites the file on its next
   change, so manual edits are silently lost.
2. **Resources are addressed by `name`**, not by id — keys, remotes and tunnels
   each have a unique name, and references between them are names.
3. **Use `--json`** on `status`, `remote list`, `key list` and `tunnel list` for
   parseable output. The `[ssh-tunnel @ …]` banner goes to stderr, so stdout is
   clean JSON.
4. **The service must be running** for resource commands. Check with
   `ssh-tunnel status`; if nothing is attached, the error says so.
5. **ssh is invoked non-interactively** (`BatchMode=yes`, password and
   keyboard-interactive auth disabled). A remote that only accepts a password
   can never work — it needs key-based auth.
6. **Confirm before destroying.** `tunnel stop`, `tunnel rm`, `remote rm`,
   `key rm`, `stop` and `uninstall` can cut connections someone is using. Do not
   touch a tunnel the user did not name.

## Orient first

```bash
ssh-tunnel status          # which instance am I attached to, is it up
ssh-tunnel tunnel list     # every tunnel with its live state / pid / error
ssh-tunnel remote list     # SSH targets tunnels can hang off
ssh-tunnel key list        # managed private keys
```

`status` reports the **scope** the CLI is talking to: `system` (root-owned
service, starts at boot) or `user` (per-user service). They are separate
instances with separate config — a tunnel added to one is invisible to the
other.

## Create a tunnel

A tunnel always references a remote, so create the remote first (reuse an
existing one when the host matches).

```bash
ssh-tunnel remote add --name prod-db --host 10.0.0.5 --port 22 --user deploy
ssh-tunnel tunnel add \
  --name prod-pg \
  --remote prod-db \
  --direction -L \
  --bind-addr 127.0.0.1 --bind-port 15432 \
  --target-host 127.0.0.1 --target-port 5432 \
  --auto-start
ssh-tunnel tunnel start prod-pg
ssh-tunnel tunnel list --json      # verify state == "running"
```

`--auto-start` makes the service bring the tunnel up at start and keep it alive;
without it the tunnel is only a definition you start by hand. Adding a tunnel
does **not** start it — always `tunnel start` and then confirm the state.

## Direction: `-L` versus `-R`

Both build the same ssh forward spec
`bind_address:bind_port:target_host:target_port`; the direction decides which
machine listens and whose network resolves the target.

| | `-L` (local forward) | `-R` (remote forward) |
| --- | --- | --- |
| Listens on | the machine running the service | the remote SSH server |
| Target is reached from | the remote SSH server | the machine running the service |
| Use it to | reach something that only the remote can see | publish something local on the remote |

Pick `-L` for "let me connect to the remote's database as if it were local".
Pick `-R` for "let the remote (or its users) reach my local dev server".

For `-R`, a `bind_address` other than `localhost` only works if the remote sshd
sets `GatewayPorts yes`; otherwise sshd silently binds loopback only.

## Keys and authentication

Remotes may bind a managed key (`remote add --key <name>`). When a remote binds
no key, behaviour depends on scope:

- **system service** — runs as root with no login session or ssh-agent, so it
  injects the managed default key (`app.system_default_key`).
- **user service** — injects no `-i` and lets ssh use your normal identities
  (`~/.ssh`, agent).

So a tunnel that worked under a user-scope service can fail after moving to
system scope: the system default key is a freshly generated one no server has
authorized yet. Either authorize it, or bind the remote to a key that is already
trusted.

```bash
ssh-tunnel key list --json                  # names + public keys
ssh-tunnel key pub <name>                   # authorized_keys line to install
ssh-tunnel key add --name deploy --source ~/.ssh/id_ed25519
ssh-tunnel remote update prod-db --key deploy
```

The public key printed by `key pub` must be appended to
`~/.ssh/authorized_keys` of the remote **user** the remote definition names.

## Diagnose a failing tunnel

`tunnel list --json` is the whole diagnosis surface: each entry carries
`state` (`running` / `stopped` / `error`), `pid`, and for failures an `error`
string the service already translated from ssh's stderr into a plain diagnosis.

```bash
ssh-tunnel tunnel list --json | jq '.[] | select(.state == "error")'
ssh-tunnel tail -n 200                      # service log, live
```

| `error` says | Do this |
| --- | --- |
| authentication failed … password/keyboard-interactive | The remote wants a password; set up key auth (`key pub` → remote `authorized_keys`). |
| authentication failed; verify keys… | Wrong or unauthorized key — check `remote list --json` for the bound key, verify it with `key pub`. |
| host key verification failed | Remote key untrusted under the current policy; verify the host, then trust it or adjust `app.ssh_host_key_policy`. |
| remote host key changed | Investigate before trusting — this is what a MITM looks like. |
| hostname could not be resolved | Fix `--host` on the remote. |
| connection refused | Wrong host/port, or sshd is down. |
| network connection failed | Remote unreachable from this machine (VPN, firewall, host offline). |

A tunnel whose bind port is already taken fails at start
(`ExitOnForwardFailure=yes`); pick another `--bind-port`.

## Choosing the instance

```bash
ssh-tunnel connect system      # pin the CLI to the system service
ssh-tunnel connect user        # pin it to the per-user service
ssh-tunnel connect --show      # what am I pinned to
ssh-tunnel --home <path> …     # one-shot override, does not change the pin
```

## Full reference

`reference.md` next to this file lists every command, every flag, the JSON
shapes and the config fields. Read it before constructing a command you have not
used here — or run `ssh-tunnel skill print --reference`. When in doubt,
`ssh-tunnel <command> --help` is authoritative for the installed version.
