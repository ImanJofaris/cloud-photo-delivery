# Deployment Guide

Production topology, release process, observability, backups, and the runbook for Cloud Photo Delivery. Written for Phase 10 (`doc/phases/Phase 10 - Hardening and Deploy.md`).

---

## 1. Topology

```text
Cloudflare (DNS, WAF, CDN, TLS)
   │
   ├── Next.js frontend (apps/web — static/edge or Node hosting)
   ├── Go API (N stateless containers)      :8080  public
   ├── Go Worker (M containers)              no public port
   │         │
   │         ├── Managed PostgreSQL 16
   │         └── Cloudflare R2
   └── Metrics scraping (internal only): API :9091, worker :9092
```

- **Stateless API.** No local files, no in-memory session state beyond the per-instance rate limiter. Add instances freely.
- **Worker.** Runs processing, lifecycle, exports, cleanup, and reconciliation jobs. Scale by adding instances; jobs are claimed with `FOR UPDATE SKIP LOCKED`.
- **Migrations are a release step,** not a boot step. Run `goose` in a separate job before rolling out new API/worker images.
- **Never proxy bytes.** Uploads go browser ⇄ R2 with presigned URLs; the gallery serves signed R2 URLs. Only `/api/v1/*` hits the API.

Container images are built from the repository `Dockerfile` (`--build-arg TARGET=api|worker`), run as the `nonroot` distroless user, and listen on `:8080` (API). The worker exposes no HTTP surface except its internal metrics listener.

---

## 2. Release pipeline

`.github/workflows/release.yml` runs on `v*` tags (or manually):

1. Builds and pushes `ghcr.io/<repo>-api` and `ghcr.io/<repo>-worker` tagged with the git tag and commit SHA.
2. Triggers `DEPLOY_WEBHOOK_URL` (if configured) with the tag; wire this to your platform's deploy hook.

CI (`.github/workflows/ci.yml`) gates every change with `go vet` (unit + integration tags), `-race` unit and integration tests, `golangci-lint`, `govulncheck`, frontend lint/typecheck/test/build, and Docker image builds.

### Release order

```text
1. Backup the database           scripts/backup.sh
2. Run migrations                goose -dir migrations postgres "$DATABASE_URL" up
3. Roll out the worker first     (it only consumes new columns/tables)
4. Roll out the API              rolling or blue/green, health-gated
5. Verify                        /healthz, /readyz, /metrics, one gallery read
```

Migrations must be backward compatible with the previous API/worker version (add columns before using them, drop later) so a rollback does not need a down-migration.

---

## 3. Environment

Every setting is read from the environment (`os.Getenv`); `.env` is **not** auto-loaded. See `.env.example` for the full annotated list.

### Required

| Variable | Notes |
|---|---|
| `APP_ENV` | `dev`, `test`, or `prod`. `prod` enables HSTS and JSON logs, and rejects wildcard CORS. |
| `DATABASE_URL` | `postgres://user:pass@host:5432/db?sslmode=require` |
| `JWT_SECRET` | Long random value; also the fallback analytics salt. Rotating it invalidates all sessions and gallery unlocks. |
| `BILLING_WEBHOOK_SECRET` | Shared secret for the provider webhook. |
| `PUBLIC_BASE_URL` | Public frontend origin, used in QR/gallery links and password-reset mails. |

### Object storage

`R2_ENDPOINT`, `R2_ACCESS_KEY`, `R2_SECRET_KEY`, `R2_BUCKET`, `R2_REGION` (`auto` for R2).

### Hardening (Phase 10)

| Variable | Default | Notes |
|---|---|---|
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated allowlist. **Must not be `*` in prod** (config fails to load). |
| `MAX_BODY_BYTES` | `1048576` | Middleware body cap; JSON handlers cap at 1 MiB regardless. |
| `HTTP_REQUEST_TIMEOUT` | `30s` | Per-request deadline; late writes are discarded and the client gets a `504 REQUEST_TIMEOUT` envelope. |
| `METRICS_ADDR` | `:9091` | API metrics listener. Empty disables. Bind to a private interface; **never expose publicly.** |
| `WORKER_METRICS_ADDR` | `:9092` | Worker metrics listener. Same rules. |

### Database tuning (Phase 10)

| Variable | Default | Notes |
|---|---|---|
| `DB_MAX_CONNS` | `10` | Keep `instances × DB_MAX_CONNS` below the PostgreSQL connection limit. |
| `DB_MIN_CONNS` | `1` | Must not exceed `DB_MAX_CONNS`. |
| `DB_CONN_MAX_LIFETIME` | `1h` | |
| `DB_CONN_MAX_IDLE_TIME` | `30m` | |
| `DB_STATEMENT_TIMEOUT` | `30s` | Server-side; `0` disables. |
| `DB_SLOW_QUERY_THRESHOLD` | `500ms` | Queries slower than this log `slow query` at WARN (SQL only, never arguments); `0` disables. |

### Auth, gallery, lifecycle, exports

`ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `PASSWORD_RESET_TTL`, `LOCKOUT_MAX_ATTEMPTS`, `LOCKOUT_DURATION`, `SIGNED_URL_TTL`, `GALLERY_UNLOCK_TTL`, `ANALYTICS_HASH_SALT`, `BILLING_PROVIDER`, `EVENT_PURGE_GRACE_DAYS`, `EXPIRY_WARN_DAYS`, `EXPORT_TTL`, `WORKER_CONCURRENCY`, `WORKER_POLL_INTERVAL`, `SHUTDOWN_TIMEOUT`.

Secrets live in the platform's secret store, never in the repository or image. The logger redacts `password*`, `*secret*`, `*token*`, `authorization`, `api_key`, `signature`, and signed-URL query parameters, plus JWT-shaped strings, as defense in depth.

---

## 4. Rate limits

Per-instance, in-memory (approximate across replicas; documented limitation). Starting values from the phase plan:

```text
Login / refresh / reset:   10 req/min/IP
Signup:                     5 req/min/IP
Upload init:              100 req/min/device or user
Upload complete:          300 req/min/user
Signed URL generation:     60 req/min/IP
Admin:                     30 req/min/user
Public gallery:           no app limit — expected behind the CDN
```

Exceeding a limit returns `429` with the standard envelope (`RATE_LIMITED`) and `Retry-After: 60`. If cross-instance accuracy is needed, replace the in-memory limiter with a shared store.

---

## 5. Observability

`GET /metrics` (Prometheus text format) is served from the dedicated internal listeners only; it is not registered on the public router (there is a test asserting this).

```text
API       http://<api-host>:9091/metrics
Worker    http://<worker-host>:9092/metrics
```

Key series:

| Metric | Meaning |
|---|---|
| `cpd_http_requests_total{method,route,status}` | Traffic and error rate by route pattern |
| `cpd_http_request_duration_seconds{method,route}` | Latency histogram (p50/p95/p99) |
| `cpd_http_in_flight_requests` | Concurrency |
| `cpd_db_pool_{max,total,acquired,idle}_connections` | Pool saturation |
| `cpd_jobs_{pending,running,failed}` | Queue depth (scrape-time query; `NaN` on error) |
| `cpd_worker_jobs_total{type,outcome}` | Job outcomes: `done`, `retry`, `failed`, `released` |
| `cpd_worker_job_duration_seconds{type}` | Job duration |
| `cpd_uploads_total{outcome}` | Upload initialized / completed / failed |
| `cpd_r2_requests_total{operation}` / `cpd_r2_errors_total{operation}` / `cpd_r2_request_duration_seconds{operation}` | Object storage health |

An example scrape configuration is in `deploy/prometheus.example.yml`.

### Alert starting points

```text
- alert: APIHigh5xx
  expr: sum(rate(cpd_http_requests_total{status=~"5.."}[5m])) / sum(rate(cpd_http_requests_total[5m])) > 0.05
  for: 10m
- alert: QueueDepthGrowing
  expr: cpd_jobs_pending > 100 and increase(cpd_jobs_pending[15m]) > 0
  for: 15m
- alert: JobFailures
  expr: sum(rate(cpd_worker_jobs_total{outcome="failed"}[15m])) > 0
  for: 10m
- alert: R2Errors
  expr: sum(rate(cpd_r2_errors_total[5m])) > 0.5
  for: 10m
- alert: DBPoolSaturation
  expr: cpd_db_pool_acquired_connections / cpd_db_pool_max_connections > 0.9
  for: 10m
- alert: StorageDrift
  expr: increase(cpd_worker_jobs_total{type="storage.reconcile",outcome="done"}[25h]) > 0
  # combine with the `storage drift detected` log line; the job reports, never repairs
```

Structured audit events (`"audit_action"` field on records with `"msg":"audit"`) cover `auth.login`, `auth.password_reset`, `device.create`, `device.rotate`, `device.revoke`, `event.delete`, and `photo.delete`. Ship them to your log platform and alert on `auth.login` failure spikes.

---

## 6. Backups

### Database

`scripts/backup.sh` takes a custom-format `pg_dump`, verifies it with `pg_restore --list`, and prunes local files older than `BACKUP_RETENTION_DAYS` (default 14):

```text
DATABASE_URL=postgres://... BACKUP_DIR=/var/backups/cpd ./scripts/backup.sh
```

Schedule it (cron/systemd timer/platform job) at least daily, store copies off-host, and test restores regularly. `scripts/restore.sh <file>` restores with `--clean --if-exists`; stop the API and worker first, restore, run migrations, then start them.

The round-trip is covered by `pkg/database/backup_integration_test.go` (dump → destroy → restore → verify) against a real PostgreSQL container.

### R2

- Enable object versioning or a lifecycle rule that retains originals for at least the event retention period plus the purge grace (`EVENT_PURGE_GRACE_DAYS`).
- Delete derivative/export/branding prefixes on a short lifecycle (they are regenerable); originals are deleted by the purge job, not by lifecycle.
- `storage.reconcile` compares DB counters with the originals in R2 daily and logs `storage drift detected`; alert on that line.

---

## 7. Runbook

### Incident response

1. Check `GET /healthz` and `GET /readyz` (readiness fails when the database is unreachable).
2. Scrape `/metrics` and look at 5xx rate, queue depth, job failures, R2 errors, and pool saturation.
3. Check logs by `request_id`; audit records identify sensitive actions.
4. If a deploy caused it, roll back (below). If infrastructure, fail over using the provider's tooling.

### Rollback

```text
1. Re-deploy the previous image tags (ghcr.io/<repo>-api:<previous>, -worker:<previous>).
2. Do NOT roll back migrations unless the release included a destructive change;
   releases must be backward compatible for one version.
3. Verify /readyz and one gallery read.
```

### Restore from backup

```text
1. Stop API and worker.
2. ./scripts/restore.sh backups/cpd-<stamp>.dump
3. goose -dir migrations postgres "$DATABASE_URL" up
4. Start worker, then API, then verify.
```

### Rotate secrets

| Secret | Procedure |
|---|---|
| `JWT_SECRET` | Update the secret and restart the API. All sessions, refresh tokens (hashed, but the access tokens are invalid) and gallery unlock tokens are invalidated; users log in again. |
| `BILLING_WEBHOOK_SECRET` | Rotate with the provider; agree on an overlap window if the provider supports two secrets, otherwise rotate during low traffic. |
| `R2_ACCESS_KEY` / `R2_SECRET_KEY` | Create a new R2 token, deploy it, then revoke the old token. |
| Database password | Rotate in the database and secret store, restart all instances. |

### Load testing

Run `k6` scenarios against staging before major events or releases. See `test/load/README.md` for usage and notes on rate limits and CDN measurement.

---

## 8. Security checklist (Phase 10 §4)

- [x] TLS everywhere; HSTS in `prod` (Cloudflare origin TLS + `SecurityHeaders`).
- [x] Secrets from env/secret store; never in repo.
- [x] Password hashing + token hashing (Phases 1/6).
- [x] No signed URLs or tokens in logs (redacting handler + unit tests).
- [x] Dependency scanning (`govulncheck` in CI; add Dependabot in the platform).
- [x] Container image scanning; non-root distroless user.
- [x] CORS allowlist enforced; wildcard rejected in `prod`.
- [x] Request body limits + server/request timeouts (slowloris protection).
- [x] Tenant isolation regression suite runs in CI (`go test -tags=integration ./...`).
- [x] Admin endpoints require explicit authorization (`RequireAdmin`, Phase 9).
- [x] Rate limits verified with tests.
- [x] Metrics kept off the public router (test asserts `/metrics` 404s there).
