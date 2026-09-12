# Developer Onboarding

Welcome. This guide gets a brand-new engineer from a fresh clone to a running system, and explains how this project is organized and how we work.

If you only read one thing, read this file top to bottom once.

---

## 1. What this project is

**Cloud Photo Delivery** is a cloud SaaS for Malaysian photobooth operators. The flow is:

```text
Take photo -> Upload to cloud -> Guest scans QR -> View/download photo
```

The API is the product. Every feature is designed and tested at the API layer first (API-first). The frontend is a consumer of the API, not the other way around.

Read these for full context:

- `doc/Product Specification.md` — what we are building and why.
- `doc/Technical Specification.md` — architecture, performance targets, stack rationale.
- `doc/Implementation Plan.md` — the master plan, all phases, conventions.
- `doc/Test Strategy.md` — how we test.
- `doc/phases/` — the plan for each individual phase.
- `doc/phases/` also contains the **completed** reports for finished phases.

---

## 2. Prerequisites

| Tool | Version | Why |
|---|---|---|
| Go | 1.24+ | Backend language |
| Docker Desktop | recent | Postgres + MinIO locally, integration tests |
| Node.js | 20+ | Frontend (Phase 5+), optional before then |
| Git | any | Version control |
| `make` | any | Task shortcuts (Git Bash, WSL, or `choco install make`) |

Optional but recommended:

| Tool | Install | Why |
|---|---|---|
| `goose` | `go install github.com/pressly/goose/v3/cmd/goose@latest` | Migrations |
| `golangci-lint` | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` | Linting |
| `gcc` (mingw-w64) | `choco install mingw` | Enables `go test -race` on Windows |

Verify:

```text
go version
docker info
```

`docker info` must print a Server Version. If it errors, start Docker Desktop first.

---

## 3. First-time setup (5 minutes)

```text
git clone <repo-url>
cd cloud-photo-delivery

copy .env.example .env      # Windows
# cp .env.example .env      # macOS/Linux

make up                     # starts Postgres + MinIO + creates the bucket
make migrate-up             # applies migrations (requires goose)
make run                    # starts the API
```

In a second terminal:

```text
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

Expected:

```json
{"data":{"status":"ok"},"error":null}
{"data":{"status":"ready"},"error":null}
```

Stop the infrastructure when done:

```text
make down
```

---

## 4. Everyday commands

```text
make up              Start Postgres + MinIO
make down            Stop containers
make run             Run the API
make worker          Run the background worker
make build           Build bin/api and bin/worker
make test            Unit tests (fast)
make test-integration Integration tests (needs Docker)
make cover           Coverage summary
make lint            go vet + golangci-lint
make fmt             Format code
make migrate-up      Apply migrations
make migrate-down    Roll back one migration
```

Direct Go commands also work:

```text
go test ./...
go test -tags=integration ./...
go test -race ./...              # needs gcc
go build ./...
```

---

## 5. How the code is organized

```text
cmd/
  api/        HTTP API entrypoint (main.go wires config, DB, router)
  worker/     Background worker entrypoint
internal/
  platform/   Cross-cutting: config, logging, apperr
  <domain>/   Business domains (auth, events, uploads, ...)
pkg/
  httpx/      Response envelope + middleware
  database/   Postgres pool helpers
  r2/         Object storage client (R2 + MinIO)
migrations/   SQL migrations (goose format)
api/          OpenAPI contract
doc/          Product/tech specs, plan, phase docs, onboarding
test/         E2E tests (added in later phases)
```

**The golden rule of structure:** business logic lives in `internal/<domain>` service structs and must be testable **without** an HTTP server or a database. Handlers only parse requests and call services. Repositories own SQL. This keeps the unit test layer fast and meaningful.

---

## 6. API conventions (locked in Phase 0)

Every response uses the same envelope:

```json
{ "data": {}, "error": null }
```

```json
{ "data": null, "error": { "code": "EVENT_NOT_FOUND", "message": "Event not found" } }
```

- Base path: `/api/v1`
- Middleware order: Request ID -> Recover -> Logging -> CORS -> (Auth from Phase 1)
- Errors are typed in `internal/platform/apperr`; never leak SQL/driver errors.
- Pagination is cursor-based for large collections. Offset pagination is banned for photos.
- Upload mutations honor `Idempotency-Key`.
- Never log passwords, tokens, API keys, or signed URLs.

---

## 7. How we work, phase by phase

We build in phases defined in `doc/Implementation Plan.md`. Each phase is a self-contained unit of value.

For every phase:

1. **Plan is already written.** Read `doc/phases/Phase N - <name>.md`.
2. **Update the OpenAPI contract first** if the phase touches the API.
3. **Implement** service -> repository -> handler.
4. **Test** at all applicable layers (unit always; integration where I/O exists; E2E for the phase's critical flow).
5. **Verify locally**: `make lint`, `make test`, `make test-integration`.
6. **Write the completion report** using `doc/phases/_TEMPLATE - Phase Report.md`.
7. **Update** `doc/Implementation Plan.md` status and the README if needed.
8. **Commit** in a focused commit (see section 9).

A phase is **not done** until its Definition of Done (in the phase plan and `doc/Contributing.md`) is fully checked.

---

## 8. Testing essentials

- Unit tests: fast, no I/O, no network. Run on every save.
- Integration tests: real Postgres + MinIO via Testcontainers. Guarded by `//go:build integration`.
- E2E tests: full flows against real binaries (added from Phase 5).

Full details in `doc/Test Strategy.md`.

Test files live next to the code (`service_test.go`, `handler_test.go`, `repository_integration_test.go`).

---

## 9. Git conventions

- Branch names: `phase-0-foundation`, `phase-1-auth`, or `feat/<short>`, `fix/<short>`.
- Commit style: imperative, scoped, e.g.

```text
phase 0: scaffold API, config, httpx, storage, CI
feat(events): add cursor pagination
fix(uploads): handle idempotency key conflict
```

- Keep commits focused. Do not commit `.env`, secrets, or build artifacts.
- Never commit directly to `main` once the team is more than one person.

---

## 10. Troubleshooting

### `make up` starts but the bucket is missing

The `createbucket` service retries on first run and may print an error before succeeding. Check:

```text
docker compose logs createbucket
```

You want to see `bucket ready`. If not, run `make down` then `make up` again.

### `bind: An attempt was made to access a socket in a way forbidden by its access permissions` (Windows)

Port 8080 is reserved by Hyper-V/WinNAT on some Windows machines. Either use another port:

```text
# in .env
HTTP_ADDR=:18080
```

or free the range (admin PowerShell):

```text
net stop winnat
net start winnat
```

### `-race requires cgo` / `gcc not found` (Windows)

Install a C compiler (`choco install mingw`) or run the race suite in CI/Linux. Tests still run without `-race`.

### Integration tests hang or fail to pull images

Ensure Docker Desktop is running and you have network access. MinIO images are pulled from `quay.io`. The first run is slow; later runs are cached.

### `goose: command not found`

```text
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH`.

### Reset the local database

```text
make down
docker volume rm cloud-photo-delivery_pgdata
make up
make migrate-up
```

---

## 11. Where to get help

- Architecture questions: `doc/Technical Specification.md`.
- Scope questions: `doc/Product Specification.md` (section 26 is the hard "do not build" list for MVP).
- Process questions: `doc/Contributing.md`.
- What was actually built so far: the phase reports in `doc/phases/`.
