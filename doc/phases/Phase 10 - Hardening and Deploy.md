# Phase 10 — Hardening, Observability & Deployment

**Goal:** Production-ready: rate limiting, observability, security hardening, CI/CD, and a repeatable deployment target on Cloudflare + containers.

**Depends on:** Phases 0–9.

**Exit criteria:** Load smoke test passes targets; dashboards/metrics live; production deploy documented and reproducible; security checklist complete.

---

## 1. Features

- Rate limiting (token bucket) per route/IP/device/API-key.
- Security headers, strict CORS, request size limits, timeouts.
- Metrics (Prometheus): request latency histogram, in-flight, DB pool, queue depth, job durations, upload success/fail, R2 errors.
- Tracing (OpenTelemetry) optional; correlation via Request ID.
- Structured audit log for sensitive actions (login, key rotation, delete).
- Load testing with `k6` against upload + gallery pipeline.
- DB tuning: connection pool sizing, statement timeouts, slow query log.
- Backups: Postgres backup/restore procedure; R2 lifecycle rules.
- CI/CD: build, test, scan (govulncheck), push images, deploy.
- Deployment guide: Cloudflare (DNS/WAF/CDN) in front of stateless Go API + worker, managed Postgres, R2.

---

## 2. Rate Limits (starting values)

```text
Login:                 10 req/min/IP
Signup:                 5 req/min/IP
Upload init:          100 req/min/device or user
Upload complete:      300 req/min/user
Public gallery:       high + CDN cache
Signed URL generation: 60 req/min/IP
Admin:                 30 req/min/user
```

Backed by an in-memory limiter per instance for MVP (documented as approximate), upgradeable to Valkey if cross-instance accuracy is required.

---

## 3. Observability

Metrics endpoint `GET /metrics` (internal only, not public). Dashboards for:

```text
p50/p95/p99 API latency
error rate by code
queue depth + job failure rate
image processing time
upload success/failure + retry counts
R2 error rate + latency
DB pool saturation + slow queries
storage used vs plan
```

Alerts:

```text
5xx rate > threshold
queue depth growing
job failures > threshold
R2 errors
DB connection saturation
```

---

## 4. Security Checklist

- [ ] TLS everywhere; HSTS.
- [ ] Secrets from env/secret store; never in repo.
- [ ] Password hashing + token hashing verified (Phases 1/6).
- [ ] No signed URLs or tokens in logs (grep-based log test).
- [ ] Dependency scanning (`govulncheck`, Dependabot).
- [ ] Container image scanning; non-root user.
- [ ] CORS allowlist, not `*`, for authenticated routes.
- [ ] Request body limits + timeouts (slowloris protection).
- [ ] Tenant isolation regression suite runs in CI.
- [ ] Admin endpoints require explicit authorization.
- [ ] Rate limits verified with tests.

---

## 5. Deployment

```text
Cloudflare (DNS, WAF, CDN, TLS)
   │
   ├── Next.js frontend (static/edge or node hosting)
   ├── Go API (N stateless containers)
   └── Go Worker (M containers)
         │
         ├── Managed PostgreSQL
         └── Cloudflare R2
```

- Stateless API: no local files, no in-memory sessions beyond cache/rate limit.
- Horizontal scale by adding API/worker instances without code changes.
- Migrations run as a release step (separate job), not on boot race.
- Blue/green or rolling deploy; health-gated.

---

## 6. Load Test Targets (from Technical Spec)

```text
API CRUD p95            < 100 ms
Gallery initial p95     < 300 ms
Upload init p95         < 100 ms
Thumbnail available     < 5 s after upload
Concurrent uploads      100+
Concurrent gallery      1,000+
Event size              50,000+ photos
Single photo            up to 100 MB
```

`k6` scenarios: `upload_flow.js`, `gallery_read.js`, `mixed.js`.

---

## 7. Test Plan

**Unit**
- [ ] rate limiter: allows under limit, blocks over, refills over time.
- [ ] security middleware: headers present, CORS allowlist enforced, body limit rejects oversized.
- [ ] log sanitizer: no tokens/URLs/passwords emitted.
- [ ] metrics middleware records latency + status.

**Integration**
- [ ] `/metrics` served and increases after requests.
- [ ] migrations run cleanly on a fresh managed-like DB.
- [ ] backup/restore script round-trips a seeded DB.

**E2E / Load**
- [ ] `k6` upload flow sustains 100 concurrent uploads with acceptable error rate.
- [ ] `k6` gallery read sustains 1,000 concurrent viewers behind cache.
- [ ] full regression suite (`unit + integration + e2e`) green in CI.

---

## 8. Definition of Done

- [ ] All master DoD items.
- [ ] Security checklist fully checked.
- [ ] Production deployment performed once end-to-end from CI.
- [ ] Runbook: incident response, rollback, restore, rotate secrets.
