# AGENTS.md

Cross-tool agent guide for this repo. The canonical instructions live in [`CLAUDE.md`](./CLAUDE.md) — read it first. This file exists so non-Claude agents (Codex, Cursor, Copilot, Aider, …) discover the same guidance.

## TL;DR

- Go daemon + Cobra CLI + Gin REST API + embedded Vue 3 SPA. See `README.md` for user-facing docs.
- Use `make` targets, not raw `go`/`pnpm`. Common ones: `make run`, `make build`, `make test`, `make ui-dev`, `make ui-build`, `make dist`.
- All state mutation flows through `internal/services` (`Registry` for persisted config, `Runtime` for live state, `Manager` for `ssh` child processes). HTTP API and CLI both call into it — never bypass.
- `ssh` is invoked non-interactively (`BatchMode=yes`, password/keyboard-interactive disabled). Password-only remotes are expected to fail fast with a diagnostic from `Manager.diagnoseSSHFailure`. Don't add interactive fallbacks.
- SPA embedding is gated by the `embedui` build tag; `make build` / `make dist` apply it. Plain `go build .` ships the placeholder page in `internal/web/embed.go`.
- API token lives in `<SSH_TUNNEL_HOME>/token`, never `config.yaml`. `/api/bootstrap` is loopback-only and exposes it to the SPA; embedded HTML additionally injects `window.__AUTH_TOKEN__`.

See `CLAUDE.md` for the full architecture walkthrough, conventions, and per-area "how to change X" notes.
