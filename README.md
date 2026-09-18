# Cloud Photo Delivery

Cloud-based photo delivery SaaS for Malaysian photobooths. See `doc/` for the Product, Technical, and phased Implementation Plan.

## Documentation

- [AGENTS.md](AGENTS.md) — operating guide: commands, layering rules, environment gotchas.
- [Developer Onboarding](doc/Developer%20Onboarding.md) — set up and run the project, project map, troubleshooting.
- [Contributing](doc/Contributing.md) — workflow, layering rules, Definition of Done.
- [Implementation Plan](doc/Implementation%20Plan.md) — all phases and conventions.
- [Test Strategy](doc/Test%20Strategy.md) — unit / integration / E2E approach.
- [Deployment](doc/Deployment.md) — topology, release process, observability, backups, runbook.
- Completion reports: all phases under [doc/phases](doc/phases) — latest: [Phase 10](doc/phases/Phase%2010%20-%20Report.md).

## Status

All phases are **complete**: backend Phases 0–10 and frontend Phases F0–F5. The API covers auth, events, uploads, processing, public galleries, devices, branding, billing, lifecycle/ZIP/analytics/admin, and Phase 10 hardening (rate limits, security headers, Prometheus metrics, audit logs, backups, load scripts). See the phase reports under `doc/phases/` and the [Deployment guide](doc/Deployment.md).

## Stack

- Go 1.26+ (`net/http` + `chi` + `pgx`)
- PostgreSQL 16
- Cloudflare R2 (S3-compatible); MinIO locally
- Next.js 16 + React + Tailwind (`apps/web`, pnpm workspace)
- Docker / docker-compose

## Run locally

Prerequisites: Go 1.26+, Docker Desktop, Node 20+ (run `corepack enable` once for pnpm), and [goose](https://github.com/pressly/goose) (`go install github.com/pressly/goose/v3/cmd/goose@latest`). `make` is optional; the commands below do not need it.

### 1. Configure

```text
copy .env.example .env      # Windows
cp .env.example .env        # macOS/Linux
```

On Windows, port 8080 is often reserved by WinNAT/Hyper-V. If `go run ./cmd/api` fails with `bind: An attempt was made to access a socket...`, set `HTTP_ADDR=:18080` in `.env` (`apps/web/.env.local` already points at `http://localhost:18080`).

### 2. Start infrastructure (Postgres + MinIO + bucket)

```text
docker compose up -d
docker compose logs createbucket   # wait for "bucket ready"
```

### 3. Apply migrations

```text
goose -dir migrations postgres "postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable" up
```

### 4. Start the API

`cmd/api` does **not** auto-load `.env`, so export the variables in each terminal first.

PowerShell:

```powershell
Get-Content .env | Where-Object { $_ -match '^\s*[^#\s][^=]*=' } | ForEach-Object {
  $name, $value = $_ -split '=', 2
  [Environment]::SetEnvironmentVariable($name.Trim(), $value.Trim(), 'Process')
}
go run ./cmd/api                   # API on :18080, metrics :9091
```

macOS/Linux: `set -a; . ./.env; set +a; go run ./cmd/api`

### 5. Start the worker (required for uploads)

In a second terminal, load `.env` as above, then:

```text
go run ./cmd/worker                # metrics :9092
```

Without the worker, uploaded photos stay `PROCESSING` forever: the API only enqueues `PROCESS_PHOTO` jobs, and the worker generates the image derivatives and marks photos `READY`. It also owns event lifecycle, exports, and storage reconciliation.

### 6. Start the dashboard

In a third terminal:

```text
pnpm install
pnpm dev                           # http://localhost:3000
```

### Verify

```text
curl.exe http://localhost:18080/healthz   # Windows
curl.exe http://localhost:18080/readyz
curl http://localhost:18080/healthz       # macOS/Linux
curl http://localhost:18080/readyz
```

Both endpoints return the `{"data":{"status":"ok"},"error":null}` / `{"data":{"status":"ready"},"error":null}` envelope. Then open http://localhost:3000 and sign up.

### Stop

Ctrl+C the three terminals, then:

```text
docker compose down
```

## Common commands

```text
make test                 # unit tests
make test-integration     # integration tests (requires Docker)
make cover                # coverage summary
make lint                 # go vet + golangci-lint (if installed)
make build                # build bin/api and bin/worker
make migrate-up           # apply migrations (requires goose)
make migrate-down         # roll back one migration
```

Frontend (`apps/web`):

```text
pnpm lint                 # eslint
pnpm typecheck            # tsc
pnpm test                 # vitest
pnpm build                # production build
pnpm gen:api              # regenerate packages/api-client from api/openapi.yaml
pnpm test:e2e             # Playwright (needs API on :18081)
```

## Layout

```text
cmd/api        HTTP API entrypoint
cmd/worker     background worker entrypoint
internal/      domain + platform packages
pkg/           reusable libraries (httpx, database, r2)
migrations/    SQL migrations
api/           OpenAPI contract
apps/web       Next.js operator dashboard + public gallery
packages/      shared UI and generated API client
```

## API conventions

All responses use the envelope:

```json
{ "data": {}, "error": null }
```

```json
{ "data": null, "error": { "code": "EVENT_NOT_FOUND", "message": "Event not found" } }
```
