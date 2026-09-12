# Phase 0 — Foundation & Scaffold

> **Status: COMPLETE.** See the [Phase 0 Report](Phase 0 - Report.md) for what was built and how it was verified.

**Goal:** A runnable, testable, deployable skeleton with no business features. Everything later builds on this.

**Depends on:** nothing.

**Exit criteria:** `make test` passes, `docker compose up` starts Postgres + MinIO, `GET /healthz` returns the standard envelope, migrations run.

---

## 1. Features

- Project layout exactly as in the master plan.
- Config loading from environment (`.env` + env vars), validated at startup (fail fast).
- Structured logging with `log/slog` (JSON in prod, text in dev).
- Standard HTTP response envelope + typed domain errors.
- Middleware: Request ID, recovery, logging, CORS, (auth added in Phase 1).
- `GET /healthz` (liveness) and `GET /readyz` (checks DB).
- `cmd/api` and `cmd/worker` entrypoints.
- `docker-compose.yml`: Postgres + MinIO + createbuckets.
- Migration tool wired and first empty baseline migration.
- Makefile + GitHub Actions CI.
- OpenAPI skeleton with health documented.

---

## 2. Deliverables

```text
go.mod
cmd/api/main.go
cmd/worker/main.go
internal/platform/config/config.go
internal/platform/logging/logging.go
internal/platform/apperr/apperr.go
pkg/httpx/response.go
pkg/httpx/middleware.go
pkg/database/pool.go
pkg/r2/client.go            # interface + MinIO/S3 impl
migrations/0001_init.sql
api/openapi.yaml
docker-compose.yml
Dockerfile
Makefile
.github/workflows/ci.yml
```

---

## 3. Key Design Details

**Config struct** (validated on load):

```go
type Config struct {
    Env          string        // dev|test|prod
    HTTPAddr     string
    DatabaseURL  string
    R2Endpoint   string
    R2AccessKey  string
    R2SecretKey  string
    R2Bucket     string
    LogLevel     string
}
```

**Error type** (`apperr`): carries `Code`, `HTTPStatus`, `Message`, optional wrapped cause. Handler layer maps it to the envelope in a single place. Unknown errors become `INTERNAL_ERROR` with a generic message.

**Storage interface** so tests can use MinIO and prod uses R2:

```go
type ObjectStore interface {
    PresignPut(ctx, key string, contentType string, ttl time.Duration) (url string, err error)
    PresignGet(ctx, key string, ttl time.Duration) (url string, err error)
    Head(ctx, key string) (size int64, err error)
    Delete(ctx, key string) error
    // multipart methods added in Phase 3
}
```

---

## 4. Tasks

1. `go mod init` and add `chi`, `pgx/v5`, `aws-sdk-go-v2` (S3/R2), `testify`, `godotenv`.
2. Implement config load + validation.
3. Implement `slog` setup and a `logging.FromContext` helper.
4. Implement `apperr` + `httpx` envelope and error mapping.
5. Implement middleware chain (Request ID, recovery, logging, CORS).
6. Wire chi router in `cmd/api` with `/healthz`, `/readyz`.
7. Add pgx pool + ping in `/readyz`.
8. Add `pkg/r2` S3-compatible client; verify with MinIO.
9. Add docker-compose (Postgres 16, MinIO, bucket autocreate).
10. Add goose/migrate and baseline migration.
11. Add Makefile targets: `run`, `worker`, `test`, `test-integration`, `lint`, `migrate-up`, `migrate-down`, `up`, `down`.
12. Add CI workflow.
13. Write `api/openapi.yaml` skeleton.

---

## 5. API Surface (Phase 0)

```http
GET /healthz   -> 200 { "data": { "status": "ok" }, "error": null }
GET /readyz    -> 200 or 503 depending on DB
```

---

## 6. Test Plan

**Unit**
- [ ] config: missing/invalid env returns error; valid env populates struct.
- [ ] httpx: success and error envelope serialization.
- [ ] apperr → HTTP status + code mapping for each domain code.
- [ ] middleware: Request ID generated and echoed; recovery turns panic into 500 envelope.
- [ ] `/healthz` handler via `httptest`.

**Integration**
- [ ] `pkg/database`: connect to test Postgres, ping succeeds.
- [ ] `pkg/r2`: presign put/get and round-trip a small object against MinIO.
- [ ] migrations: up then down leaves a clean schema.

**E2E**
- [ ] Boot `cmd/api` in a container/test process against compose services; `GET /readyz` returns 200.

---

## 7. Definition of Done

- [ ] All master Definition of Done items.
- [ ] `docker compose up -d` then `make migrate-up` succeeds on a clean machine.
- [ ] CI green on a fresh clone.
- [ ] No secrets committed; `.env.example` provided.
