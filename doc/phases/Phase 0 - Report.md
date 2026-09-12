# Phase 0 — Foundation & Scaffold — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Go version used:** 1.26.5 (target 1.24+)

---

## 1. Summary

Phase 0 delivered the runnable, testable, deployable skeleton of the backend. No user-facing features were built. The purpose was to lock in conventions (response envelope, error handling, logging, config, storage abstraction, CI gates) that every later phase builds on.

All exit criteria were met and verified against real Docker services (Postgres + MinIO).

---

## 2. Exit criteria — verification

| Criteria | Result |
|---|---|
| `make test` passes | PASS |
| `docker compose up` starts Postgres + MinIO | PASS (both healthy, bucket created) |
| `GET /healthz` returns standard envelope | PASS (live, HTTP 200) |
| `GET /readyz` checks DB | PASS (live, HTTP 200 with real DB) |
| Migrations run | PASS (integration test applies `0001_init.sql`) |
| `go build`, `go vet`, `go vet -tags=integration` | PASS |
| CI workflow committed | PASS |
| No secrets committed; `.env.example` provided | PASS |

Live verification output:

```text
GET /healthz -> 200 {"data":{"status":"ok"},"error":null}
GET /readyz  -> 200 {"data":{"status":"ready"},"error":null}
X-Request-Id header present on every response
```

---

## 3. What was built

### 3.1 Platform packages (`internal/platform/`)

| Package | Responsibility |
|---|---|
| `config` | Loads env vars, applies defaults, validates at startup (fail fast). |
| `logging` | `log/slog` setup; JSON in prod, text in dev; context-aware logger. |
| `apperr` | Typed domain errors with `Code`, `HTTPStatus`, `Message`, cause wrapping. |

### 3.2 HTTP kit (`pkg/httpx/`)

| File | Responsibility |
|---|---|
| `response.go` | `{data,error}` envelope; `Success`, `Fail`, `Error`; hides internal errors. |
| `middleware.go` | Request ID, Logging, Recover, CORS. Logger is nil-safe. |
| `server.go` | `http.Server` with sane timeouts. |

Middleware order: **Request ID -> Recover -> Logging -> CORS** (Auth added in Phase 1).

### 3.3 Data & storage

| Package | Responsibility |
|---|---|
| `pkg/database` | pgx pool with pool limits, lifetime, ping on init. |
| `pkg/r2` | `ObjectStore` interface + S3-compatible client (`aws-sdk-go-v2`). Works with R2 and MinIO. Includes `PresignPut/PresignGet/Head/Delete/Put/Get`. |

### 3.4 Entrypoints (`cmd/`)

| Binary | Responsibility |
|---|---|
| `cmd/api` | Wires config, DB, router. Routes: `/healthz`, `/readyz`. Graceful shutdown. |
| `cmd/worker` | Placeholder worker with signal handling and heartbeat. Real jobs in Phase 4. |

### 3.5 Infrastructure & tooling

| File | Purpose |
|---|---|
| `docker-compose.yml` | Postgres 16 + MinIO (`quay.io/minio/minio`) + bucket autocreate. |
| `Dockerfile` | Multi-stage build -> distroless non-root image. `TARGET` arg builds api or worker. |
| `Makefile` | `up/down/run/worker/build/test/test-integration/cover/lint/fmt/migrate-up/migrate-down`. |
| `.env.example` | All required env vars with local defaults. |
| `.golangci.yml` | Linter config (govet, errcheck, staticcheck, goimports, bodyclose, etc). |
| `migrations/0001_init.sql` | Baseline migration (pgcrypto + baseline table), goose format. |
| `api/openapi.yaml` | OpenAPI 3.1 skeleton documenting health + the shared envelope schema. |
| `.github/workflows/ci.yml` | vet, unit tests (`-race`), build, integration tests, lint. |

---

## 4. Files created

```text
.env.example
.gitignore
.golangci.yml
Dockerfile
Makefile
README.md
docker-compose.yml
go.mod
go.sum

.github/workflows/ci.yml

api/openapi.yaml

cmd/api/main.go
cmd/api/main_test.go
cmd/worker/main.go

internal/platform/apperr/apperr.go
internal/platform/apperr/apperr_test.go
internal/platform/config/config.go
internal/platform/config/config_test.go
internal/platform/logging/logging.go
internal/platform/logging/logging_test.go

pkg/database/pool.go
pkg/database/pool_integration_test.go
pkg/httpx/httpx_test.go
pkg/httpx/middleware.go
pkg/httpx/response.go
pkg/httpx/server.go
pkg/r2/s3.go
pkg/r2/s3_integration_test.go
pkg/r2/store.go

migrations/0001_init.sql
migrations/migrations_integration_test.go
```

---

## 5. Tests

### Unit (no I/O, run everywhere)
- `config`: valid load, missing `DATABASE_URL`, invalid `APP_ENV`, defaults, storage validation.
- `logging`: level mapping, context default, `With`/`FromContext` round-trip.
- `apperr`: error string, cause wrapping, `errors.Is` by code, code/status table.
- `httpx`: success/fail envelope, app-error mapping, internal-error hiding, Request ID generate/echo/preserve, panic recovery, CORS preflight + allowlist, logging injection.
- `cmd/api`: `/healthz` returns 200 + envelope, `/readyz` returns 503 when DB down, Request ID header.

### Integration (`//go:build integration`, Testcontainers)
- `pkg/database`: connect + ping + query against real Postgres; invalid DSN rejected.
- `pkg/r2`: bucket create + Put/Head/Get/Presign/Delete round-trip against real MinIO; `ErrNotFound` after delete.
- `migrations`: applies the Up section of `0001_init.sql` and asserts the table exists.

### Verification commands and results

```text
gofmt -l .                          -> clean
go build ./...                      -> OK
go vet ./...                        -> OK
go vet -tags=integration ./...      -> OK
go test ./...                       -> all packages PASS
go test -tags=integration ./...     -> all packages PASS
docker compose up -d                -> postgres healthy, minio healthy, bucket ready
live curl /healthz                  -> 200 {"data":{"status":"ok"},"error":null}
live curl /readyz                   -> 200 {"data":{"status":"ready"},"error":null}
```

`go test -race` was not run locally (no gcc on the dev Windows machine); CI runs it on Linux.

---

## 6. Fixes made during verification

| Issue | Fix |
|---|---|
| Migration test used wrong relative path | Read `0001_init.sql` from the package directory. |
| `minio/minio` image pull denied | Switched to `quay.io/minio/minio` and `quay.io/minio/mc` (compose + tests). |
| Flaky `mc` sidecar bucket creation | Create the bucket from the test process via the S3 SDK (`CreateBucket`). |
| Nil logger panic in middleware | Made `Logging`, `Logger`, and `Error` nil-safe with `slog.Default()` fallback. |

---

## 7. Known limitations / notes for Phase 1

- Port 8080 can be blocked by WinNAT on Windows; use `HTTP_ADDR=:18080` if `bind` fails.
- No auth yet — `/readyz` and `/healthz` are the only routes.
- `cmd/worker` only heartbeats; the jobs table and real handlers arrive in Phase 4.
- CORS is currently `*`; tighten the allowlist before production (Phase 10).
- Rate limiting is not present yet (Phase 10).
- The `ObjectStore` interface intentionally has no multipart methods yet; add them in Phase 3.

---

## 8. How to run it (quick reference)

```text
copy .env.example .env
make up
make migrate-up
make run
curl http://localhost:8080/healthz
```

Full instructions and troubleshooting: `doc/Developer Onboarding.md`.
