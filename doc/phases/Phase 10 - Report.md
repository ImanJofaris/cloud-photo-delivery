# Phase 10 — Hardening, Observability & Deployment — Completion Report

**Status:** COMPLETE (load execution pending staging)
**Completed:** 2026-09-16
**Author:** opencode

---

## 1. Summary

Phase 10 turned the feature-complete backend into an operable production service. The API now enforces per-route rate limits, security headers, configurable CORS allowlists, request body caps, and per-request timeouts; the logger redacts secrets; sensitive actions emit structured audit events. Both binaries expose Prometheus metrics on dedicated internal listeners covering HTTP latency/errors, DB pool saturation, queue depth, job outcomes, upload outcomes, and R2 errors/latency. The database pool is configurable with a server-side statement timeout and slow-query logging, one-command backup/restore scripts are covered by a containerized round-trip test, `k6` scenarios encode the load targets, CI adds `govulncheck` and image builds, a tag-driven release workflow pushes images, and `doc/Deployment.md` documents topology, releases, observability, backups, and the incident/rollback/secret-rotation runbook.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Rate limiting (token bucket) per route/IP/device/API-key | PASS | `cmd/api/main.go` limiters: login/refresh/reset 10/min/IP, signup 5/min/IP, upload init 100/min/device-or-user, upload complete 300/min/user, signed URL 60/min/IP, admin 30/min/user; `pkg/httpx/ratelimit.go`; tests `TestRateLimiter_RefillsOverTime`, `TestRateLimiter_MiddlewareEnvelope`, `TestRouter_*` |
| Security headers, strict CORS, body limits, timeouts | PASS | `pkg/httpx/security.go` (`SecurityHeaders`, `MaxBytes`, `RequestTimeout`), `httpx.DecodeJSON*` maps oversized bodies to `413 PAYLOAD_TOO_LARGE`; prod rejects wildcard CORS (`TestLoad_ProdRejectsWildcardCORS`); tests `TestSecurityHeaders_*`, `TestMaxBytes_*`, `TestRequestTimeout_*` |
| Prometheus metrics (latency, in-flight, DB pool, queue, jobs, uploads, R2) | PASS | `internal/platform/metrics`, `internal/jobs/metrics.go`; unit tests assert counters/gauges; integration `TestRegisterMetrics_ReportsQueueDepth`; `/metrics` served on `METRICS_ADDR` (:9091 API) and `WORKER_METRICS_ADDR` (:9092 worker), not on the public router (`TestRouter_MetricsNotExposedPublicly`) |
| Structured audit log for sensitive actions | PASS | `internal/platform/audit`; wired to `auth.login` (success/failure), `auth.password_reset`, `device.create/rotate/revoke`, `event.delete`, `photo.delete`; tests `TestRecord_*` prove structure and redaction |
| No signed URLs or tokens in logs | PASS | Redacting slog handler (`internal/platform/logging/redact.go`) scrubs sensitive keys, JWTs, and signed-URL query params; tests `TestRedact_SensitiveKeys`, `TestRedact_JWTInStringValue`, `TestRedact_SignedURLParams` |
| Load testing with k6 | PARTIAL | `test/load/upload_flow.js`, `gallery_read.js`, `mixed.js` + README encode the §6 targets and thresholds; **not executed on this host** (k6 not installed; targets require staging/Docker sizing). See §9. |
| DB tuning: pool sizing, statement timeouts, slow query log | PASS | `pkg/database` `Options` + `applyPoolOptions` (unit-tested), `NewSlowQueryTracer` (unit-tested, never logs arguments); integration `TestPool_StatementTimeoutEnforced` (`SELECT pg_sleep` fails under a 100 ms timeout) |
| Backups: Postgres backup/restore procedure | PASS | `scripts/backup.sh` (custom-format dump + `pg_restore --list` verification + retention prune), `scripts/restore.sh`; integration `TestBackupRestoreRoundTrip` (seed → dump → drop → restore → verify) |
| CI/CD: build, test, scan, push images | PASS | `.github/workflows/ci.yml` adds `go vet -tags=integration`, `govulncheck` job, Docker image builds for api+worker; `.github/workflows/release.yml` builds/pushes GHCR images on `v*` and triggers the deploy webhook; Dockerfile already non-root distroless |
| Deployment guide + runbook | PASS | `doc/Deployment.md` (topology, release order, env reference, rate limits, metrics/alerts, backups, incident/rollback/restore/rotate runbook, security checklist); `deploy/prometheus.example.yml` |
| Security checklist fully checked | PASS | Each item mapped in `doc/Deployment.md` §8; wildcard CORS rejected in prod, admin authz, tenant isolation suite in CI, `/metrics` internal-only |
| Production deployment performed once from CI | NOT DONE | Requires real infrastructure credentials; workflow + runbook are ready. Follow-up. |

---

## 3. What was built

### 3.1 Security middleware (`pkg/httpx`)

| File | Responsibility |
|---|---|
| `security.go` | `SecurityHeaders(prod)` (nosniff, frame deny, referrer policy, COOP, permissions policy, HSTS in prod); `MaxBytes(n)` backstop using `http.MaxBytesReader`; `RequestTimeout(d)` with a race-safe writer that returns a `504 REQUEST_TIMEOUT` envelope and discards late writes |
| `decode.go` | Shared `DecodeJSON` / `DecodeJSONStrict`; oversized bodies map to `413 PAYLOAD_TOO_LARGE`, all eight domain handlers migrated to it |
| `middleware.go` | CORS now consumes the configured allowlist; unknown origins get no CORS headers |
| `response.go` | `context.DeadlineExceeded` maps to `504 REQUEST_TIMEOUT` instead of a generic 500 |

Route wiring in `cmd/api/main.go`: RequestID → Recover → Logging → Metrics → SecurityHeaders → CORS → MaxBytes → RequestTimeout.

### 3.2 Rate limits (`cmd/api/main.go`)

Per-instance token buckets (`golang.org/x/time/rate`) keyed by IP, user, or device, with the §2 starting values. Uploads use a composite key: device ID when a device key is presented, otherwise user ID, otherwise IP. Verified by unit tests plus router-level tests.

### 3.3 Metrics (`internal/platform/metrics`, `internal/jobs`, `pkg/database`)

- `Metrics` owns a private `prometheus.Registry` exposing `cpd_http_requests_total`, `cpd_http_request_duration_seconds`, `cpd_http_in_flight_requests`, `cpd_uploads_total`, `cpd_worker_jobs_total`, `cpd_worker_job_duration_seconds`, and `cpd_r2_{requests_total,errors_total,request_duration_seconds}`.
- `httpx.Metrics(observer)` records chi route patterns (bounded labels), in-flight counts, and status codes.
- `metrics.WrapStore` decorates `r2.ObjectStore` with per-operation latency/error metrics (13 methods, pass-through errors).
- `metrics.RegisterDBPool` exposes max/total/acquired/idle connections via scrape-time `GaugeFunc`s.
- `jobs.RegisterMetrics` exposes pending/running/failed queue depth (NaN on query error); the poller gained `WithObserver` and reports `done`/`retry`/`failed`/`released`.
- `uploads.Service.WithObserver` reports `initialized`/`completed`/`failed`.
- Both binaries serve `/metrics` on a dedicated internal listener (`httpx.NewInternalServer`) and shut it down gracefully.

### 3.4 Logging & audit (`internal/platform/logging`, `internal/platform/audit`)

- `logging.Redact` wraps both text and JSON handlers: sensitive keys → `[REDACTED]`; string values are scrubbed of JWTs and `X-Amz-Signature`/`X-Amz-Credential`/`token=` style query parameters. Applied recursively to groups and `WithAttrs`.
- `audit.Record(ctx, action, attrs...)` emits `msg=audit` records with `audit_action`; wired into login, password reset, device create/rotate/revoke, event delete, and photo delete.

### 3.5 Database (`pkg/database`, `internal/platform/config`)

- `Options` (max/min conns, lifetimes, statement timeout, slow query threshold) applied by `applyPoolOptions`; `New` keeps the previous defaults, `NewWithOptions` is used by both binaries via `cfg.DatabaseOptions(log)`.
- `NewSlowQueryTracer` logs `slow query` at WARN with duration and SQL (arguments never logged).
- Config adds `DB_*`, `CORS_ALLOWED_ORIGINS`, `MAX_BODY_BYTES`, `HTTP_REQUEST_TIMEOUT`, `METRICS_ADDR`, `WORKER_METRICS_ADDR`, all validated (prod wildcard CORS and invalid pool bounds are rejected).

### 3.6 Backups, load tests, CI/CD, deployment

- `scripts/backup.sh` / `scripts/restore.sh` (custom format, ownership/ACL stripped, retention pruning, restore requires stopping API/worker).
- `test/load/*.js` with thresholds matching the phase targets and a README covering rate-limit and CDN caveats.
- `.github/workflows/ci.yml`: added integration-tag vet, `govulncheck`, and api/worker image builds. `.github/workflows/release.yml`: GHCR publish on tags + deploy webhook.
- `doc/Deployment.md`, `deploy/prometheus.example.yml`, README status refresh, and docker-compose metrics ports (:9091/:9092).

---

## 4. Database changes

None. Phase 10 adds no migrations; statement timeouts are connection parameters and all metrics are derived at runtime.

---

## 5. API surface added

No public API changes; `api/openapi.yaml` was not modified and the generated client is unchanged. One operational endpoint was added on a separate internal listener:

```http
GET :9091/metrics   # API process
GET :9092/metrics   # worker process
```

`GET /metrics` on the public router returns 404 by design (asserted in `TestRouter_MetricsNotExposedPublicly`).

---

## 6. Files created / modified

```text
cmd/api/main.go
cmd/api/main_test.go
cmd/worker/main.go
internal/auth/handler.go
internal/billing/handler.go
internal/devices/handler.go
internal/events/handler.go
internal/gallery/handler.go
internal/photos/handler.go
internal/uploads/handler.go
internal/uploads/service.go
internal/uploads/service_test.go
internal/users/branding_handler.go
internal/users/handler.go
internal/jobs/observer.go
internal/jobs/metrics.go
internal/jobs/metrics_integration_test.go
internal/jobs/worker.go
internal/jobs/worker_test.go
internal/platform/audit/audit.go
internal/platform/audit/audit_test.go
internal/platform/config/config.go
internal/platform/config/config_test.go
internal/platform/logging/logging.go
internal/platform/logging/redact.go
internal/platform/logging/redact_test.go
internal/platform/metrics/metrics.go
internal/platform/metrics/pool.go
internal/platform/metrics/r2store.go
internal/platform/metrics/metrics_test.go
pkg/database/pool.go
pkg/database/tracer.go
pkg/database/pool_test.go
pkg/database/pool_integration_test.go
pkg/database/backup_integration_test.go
pkg/httpx/security.go
pkg/httpx/security_test.go
pkg/httpx/decode.go
pkg/httpx/metrics.go
pkg/httpx/metrics_test.go
pkg/httpx/middleware.go
pkg/httpx/response.go
pkg/httpx/ratelimit_test.go
pkg/httpx/httpx_test.go
pkg/httpx/server.go
scripts/backup.sh
scripts/restore.sh
test/load/upload_flow.js
test/load/gallery_read.js
test/load/mixed.js
test/load/README.md
deploy/prometheus.example.yml
doc/Deployment.md
README.md
docker-compose.yml
.env.example
.github/workflows/ci.yml
.github/workflows/release.yml
go.mod / go.sum   # github.com/prometheus/client_golang
```

---

## 7. Tests

### Unit

- `pkg/httpx`: security headers (prod/dev), body-limit error, timeout envelope, late-write discard, early-response preservation, deadline→504 mapping; metrics middleware route labels/status/in-flight/unmatched; rate-limiter refill + 429 envelope.
- `internal/platform/logging`: sensitive keys, JWT strings, signed-URL params, query key/values, `WithAttrs`, groups, `Enabled` delegation.
- `internal/platform/audit`: structured event fields, redaction of secrets, `RecordWithLogger`.
- `internal/platform/metrics`: counters by route/status, in-flight, uploads/jobs, `/metrics` handler output, DB pool gauges, R2 wrapper successes/errors/`Bucket` passthrough/nil handling.
- `internal/platform/config`: hardening defaults, CORS list parsing, prod wildcard rejection, pool bounds, body/timeout validation.
- `pkg/database`: default options, option application, statement-timeout mapping, tracer fast/slow/context behavior, SQL truncation.
- `internal/jobs`: poller reports `done` and `retry` outcomes to the observer.
- `internal/uploads`: service reports `initialized`/`completed`/`failed`; nil observer is safe.
- `cmd/api`: security headers on the real router, oversized body → 413, `/metrics` absent publicly, existing health/auth tests updated for the new `NewRouter` signature.

### Integration (`//go:build integration`)

- `TestRegisterMetrics_ReportsQueueDepth` (Postgres): empty baseline, then pending/running gauges after enqueue and claim.
- `TestPool_StatementTimeoutEnforced` (Postgres): `SELECT pg_sleep(1)` aborts under a 100 ms statement timeout.
- `TestBackupRestoreRoundTrip` (Postgres): dump/verify/drop/restore/verify via the same pg_dump/pg_restore commands the scripts wrap.
- All pre-existing integration and E2E suites still pass.

### Verification commands and results

```text
gofmt -l .                          -> no output
go build ./...                      -> OK
go vet ./...                        -> OK
go vet -tags=integration ./...      -> OK
go test ./...                       -> all packages ok
go test -tags=integration -p 1 ./... -> all packages ok except two Docker flakes
                                       (internal/users lockout, pkg/r2 round trip);
                                       both pass in isolation on rerun
pnpm lint / typecheck / test / build -> not run; no API or web changes in this phase
```

Load scripts were not executed (k6 not installed on this host); they are ready for staging.

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Timeout middleware could race the handler and leak a late write into the response | `timeoutWriter` checks the request deadline under its mutex; post-deadline writes return `http.ErrHandlerTimeout` and are discarded |
| JSON handlers returned `422` for oversized bodies and decoded with a hard-coded 1 MiB limit each | Added `httpx.DecodeJSON`/`DecodeJSONStrict` mapping `*http.MaxBytesError` to `413 PAYLOAD_TOO_LARGE`; migrated all eight handlers; middleware-level `MAX_BODY_BYTES` caps every route |
| `store` decorator dropped `ListObjects`, which `admin.ReconcileHandler` needs | Reconciliation keeps the concrete store; all interface consumers use the instrumented one (documented in `cmd/worker/main.go`) |
| Backup integration test used the host-mapped DSN inside the container | Exec scripts use the container-internal DSN; output assertion tolerates the stdcopy multiplexing header |
| `context.DeadlineExceeded` surfaced as `500 INTERNAL_ERROR` | `httpx.Error` maps it to `504 REQUEST_TIMEOUT` (test updated) |
| Route labels for metrics could be unbounded if paths were used | Metrics middleware records chi route patterns, `unmatched` for 404s |

---

## 9. Known limitations / follow-ups

- **Load test execution pending.** k6 is not installed locally; the scripts and thresholds are ready. Run `test/load/upload_flow.js` and `gallery_read.js` against staging (with raised rate limits) and record results.
- **Production deployment not yet performed.** `release.yml` and the runbook are ready; the deploy webhook secret and production environment must be configured, then one end-to-end deploy should be executed from a tag.
- **Rate limits are per-instance** and approximate across replicas (as specified for the MVP). Upgrade to a shared store (Valkey) if cross-instance accuracy is required.
- **`govulncheck` runs in CI, not locally** (tool not installed on the dev host).
- **OpenTelemetry tracing** was explicitly optional in the phase plan and is not implemented; correlation is via request IDs and structured logs.
- **Dashboards** are described with example Prometheus scrape/alert config; no hosted dashboard JSON is checked in.
- On this Windows host a fully parallel Testcontainers run intermittently fails to create the reaper/provider (pre-existing, documented in Phase 9); `-p 1` runs cleanly per package.

---

## 10. How to try it

```powershell
docker compose up -d
goose -dir migrations postgres "$env:DATABASE_URL" up

# API + metrics
$env:HTTP_ADDR = ":18080"
go run ./cmd/api        # separate terminal

curl.exe -s http://localhost:18080/healthz
curl.exe -s http://localhost:18080/api/v1/auth/signup -H "Content-Type: application/json" `
  --data-binary '{"email":"p10@example.com","password":"password123","businessName":"P10"}'

# Metrics (internal listener, not the public router)
curl.exe -s http://localhost:9091/metrics | Select-String "cpd_http_requests_total"

# Security headers
curl.exe -s -D - -o NUL http://localhost:18080/healthz

# Worker + queue metrics
go run ./cmd/worker     # separate terminal
curl.exe -s http://localhost:9092/metrics | Select-String "cpd_jobs_pending"

# Backup and restore (needs pg_dump/pg_restore on PATH)
$env:DATABASE_URL = "postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable"
bash scripts/backup.sh
bash scripts/restore.sh backups/cpd-<stamp>.dump

# Load smoke (needs k6 and a running API)
k6 run -e BASE_URL=http://localhost:18080 test/load/upload_flow.js
```
