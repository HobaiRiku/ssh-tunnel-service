# ssh-tunnel-service

Cross-platform Go backend service for managing reusable SSH tunnel definitions with auto `ssh -R` / `ssh -L` launch support.

## Included architecture

- Base business service (`internal/service`) with in-memory `remote` and `commd` objects
- CLI entrypoint (`cmd/ssh-tunnel-service`) for serving API, daemon operations, and AI skill output
- HTTP API (`internal/httpapi`) for remotes/commds management and launch/stop actions
- Embedded PWA web UI (Mermaid topology diagram)
- Cross-platform daemon manager abstraction (`internal/daemon`)

## Quick start

```bash
go run ./cmd/ssh-tunnel-service serve --addr :8080
```

Open: `http://localhost:8080`

## API examples

Create a remote:

```bash
curl -X POST http://localhost:8080/api/remotes \
  -H 'content-type: application/json' \
  -d '{"id":"r1","name":"prod","host":"ssh.example.com","port":22,"user":"ubuntu"}'
```

Create a commd (`-L`):

```bash
curl -X POST http://localhost:8080/api/commds \
  -H 'content-type: application/json' \
  -d '{"id":"c1","name":"db","remoteId":"r1","direction":"-L","bindAddress":"127.0.0.1","bindPort":15432,"targetHost":"127.0.0.1","targetPort":5432,"autoStart":true}'
```

Launch:

```bash
curl -X POST http://localhost:8080/api/commds/c1/launch
```

## AI-ready CLI skill

```bash
go run ./cmd/ssh-tunnel-service skill
```
