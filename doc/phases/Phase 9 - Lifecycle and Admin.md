# Phase 9 — Lifecycle, ZIP, Analytics & Admin

**Goal:** Automate event lifecycle (expiry, soft/hard delete, reconciliation), async bulk ZIP downloads, event analytics, and the SaaS admin API.

**Depends on:** Phases 3–8.

**Exit criteria:** Events expire and delete safely via workers; ZIP download job works; analytics and admin endpoints return correct aggregates.

---

## 1. Features

### Event lifecycle
- Soft delete with configurable grace period, then permanent delete job (`event.purge`).
- Expiry scheduler job (`event.expire`) based on `expires_at`.
- Expiry warning notifications (e.g. 7 days before).
- Extend event endpoint.
- Hard delete removes DB rows and R2 objects in the correct order.

### Bulk ZIP download
- `POST /events/{id}/exports` creates a `zip.generate` job.
- Worker streams R2 objects into a ZIP, stores in R2, returns signed URL.
- Export expires after 24 hours; cleanup job removes stale exports.
- Export status endpoint for polling.

### Analytics
- Track: total photos, gallery views, unique visitors, downloads, QR scans.
- Event analytics endpoint + operator dashboard aggregates.
- Recorded via lightweight event counters (incremented on public API hits; QR scans on gallery open with `?src=qr`).

### Admin API
- Totals: users, events, photos, storage, revenue, subscriptions.
- Tenant list/inspection, subscription status.
- System health summary (queue depth, failed jobs).

### Storage reconciliation
- Periodic `storage.reconcile` job compares DB totals vs R2.

---

## 2. Database (migration `0010_lifecycle.sql`)

```sql
CREATE TABLE event_analytics (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    gallery_views BIGINT NOT NULL DEFAULT 0,
    unique_visitors BIGINT NOT NULL DEFAULT 0,
    downloads BIGINT NOT NULL DEFAULT 0,
    qr_scans BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (event_id, day)
);

CREATE TABLE exports (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,        -- pending|processing|ready|failed|expired
    object_key TEXT,
    file_size BIGINT,
    error_message TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE webhook_events (          -- provider idempotency (billing)
    id TEXT PRIMARY KEY,
    provider VARCHAR(30) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## 3. API Surface

```http
POST   /api/v1/events/{eventID}/exports
GET    /api/v1/events/{eventID}/exports/{exportID}
POST   /api/v1/events/{eventID}/extend

GET    /api/v1/events/{eventID}/analytics
GET    /api/v1/account/analytics

GET    /api/v1/admin/stats
GET    /api/v1/admin/users?cursor=
GET    /api/v1/admin/subscriptions?cursor=
GET    /api/v1/admin/health
```

Admin routes require an admin role/allowlist (add `users.is_admin` or an `admins` table).

---

## 4. New Job Types

```text
event.expire       -> mark completed/expired, notify
event.purge        -> permanent delete (DB + R2)
zip.generate       -> stream photos into ZIP, store, set signed URL
export.cleanup     -> delete expired ZIPs
storage.reconcile  -> compare DB vs R2, report/fix drift
analytics.rollup   -> optional nightly aggregation
```

---

## 5. Rules

- Deletion order: revoke public access → soft delete (already) → grace period → purge R2 objects → delete DB rows → adjust storage counters.
- Purge is idempotent and resumable (track per-object progress so a crash resumes).
- ZIP job must stream; never load all photos in memory.
- Export URLs signed + expiring; never public.
- Analytics increments are cheap upserts; avoid hot-row contention by batching if needed.
- Admin endpoints paginate and never leak PII beyond what's needed.

---

## 6. Test Plan

**Unit**
- [ ] expiry selection by date; extend updates `expires_at` and re-activates.
- [ ] purge order + idempotency; storage counters decrement once.
- [ ] export: job payload, status transitions, expiry logic.
- [ ] analytics increment/aggregate math.
- [ ] admin aggregates computed correctly from fixtures.
- [ ] handler envelope/status; admin auth required.

**Integration**
- [ ] purge against MinIO + Postgres removes all keys and rows.
- [ ] ZIP worker produces a valid archive containing the expected number of entries.
- [ ] expiry worker marks/notifies the right events.
- [ ] reconciliation detects an injected drift.
- [ ] analytics upsert increments per day.

**E2E**
- [ ] create event with short retention → advance clock → expire → grace → purge → gallery 404 and R2 empty.
- [ ] request export → worker → download ZIP → entries match photos → expires after 24h.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents exports, analytics, admin.
- [ ] Admin authorization tested (non-admin → 403).

---

## 8. Delivery Slices

Phase 9 ships in four independently verifiable slices, each with its own migration,
OpenAPI update, tests, and gates. The Phase 9 report is written after the last slice.

| Slice | Scope | Migration |
|---|---|---|
| 9A | Lifecycle & extend: `expired` status, retention-derived expiry, `event.expire`, expiry warnings, `event.purge` (grace), `POST /events/{id}/extend` | `0010_lifecycle.sql` |
| 9B | Bulk ZIP exports: `exports` table, `zip.generate`, `export.cleanup`, export status API | `0011_exports.sql` |
| 9C | Analytics: `event_analytics` + `event_visitors`, gallery counters, event/account analytics endpoints | `0012_analytics.sql` |
| 9D | Admin API & reconciliation: `users.is_admin`, `RequireAdmin`, `/admin/*`, `storage.reconcile` | `0013_admin.sql` |

**Status:** 9A complete (2026-09-13) with unit, integration, and E2E coverage plus a
reversible `0010_lifecycle.sql`. 9B complete (2026-09-13) with unit, integration, and
E2E coverage plus a reversible `0011_exports.sql`. 9C complete (2026-09-13) with unit,
integration, and E2E coverage plus a reversible `0012_analytics.sql`. 9D complete
(2026-09-14) with unit, integration, and E2E coverage plus a reversible
`0013_admin.sql`. **Phase 9 is complete** — see `Phase 9 - Report.md`.

### Locked decisions

- **Expiry status:** add `expired` to `events.status`; gallery already 404s on
  `expires_at`, so access is unaffected. Extend re-activates (`expired → active`).
- **Retention:** when `expiresAt` is omitted at creation it derives from the plan's
  `retentionDays` (`0` = never). Existing null-expiry events stay never-expire.
- **Purge trigger:** `event.purge` selects soft-deleted events past the grace period
  **or** expired events past the grace period. Order: delete R2 objects → delete DB
  rows → decrement `users.storage_bytes`. Idempotent and resumable by re-reading keys.
- **Scheduling:** no external cron. A worker-side ticker enqueues `event.expire` and
  `event.purge` every 15 minutes; `EnqueueUnique` dedupes against pending/running jobs.
  `export.cleanup` runs hourly and `storage.reconcile` daily (9B/9D).
- **ZIP strategy (9B):** `archive/zip` to a temp file (bounded memory), uploaded with a
  new streaming `PutReader`; no in-memory archives.
- **Unique visitors (9C):** exact daily uniques via `event_visitors(event_id, day,
  visitor_hash)` where the hash is salted `IP+UA`; no PII stored. Hashes are
  HMAC-SHA256 keyed by `ANALYTICS_HASH_SALT` (falls back to `JWT_SECRET`).
- **Admin auth (9D):** `users.is_admin` column, checked by a `RequireAdmin` middleware
  after `RequireAuth`; non-admin → `FORBIDDEN` (403).
- **Doc drift:** the planned `webhook_events` table is already covered by
  `billing_webhook_events` (`0009_billing.sql`); no duplicate table.
- **`analytics.rollup`:** skipped — per-day counter rows make a separate rollup job
  redundant.

### Slice 9A details

- `events` gains `expiry_warned_at` and a partial `idx_events_expires_at` index.
- Transitions added: `upcoming|active|completed → expired`, `expired → active`.
- `event.expire` marks due events and emits one warning per event inside
  `EXPIRY_WARN_DAYS` (default 7). Warnings go through an `events.Notifier` interface;
  the worker wires a log-based notifier until a real mailer exists.
- `event.purge` grace is `EVENT_PURGE_GRACE_DAYS` (default 30) and is enforced by a
  worker-side ticker; handlers are batch-loop and safe to re-run.
- `POST /api/v1/events/{eventID}/extend` body `{"days":30}` (1–3650), extends from
  `max(now, expires_at)` and re-activates expired events.
- Tests: unit (selection, warnings, extend, purge counters, scheduler), integration
  (purge against Postgres + MinIO, expiry marking, extend), E2E (short retention →
  backdated expiry → expire → grace → purge → gallery 404 + R2 empty).

### Slice 9C details

- `event_analytics(event_id, day, ...)` holds per-day counters and
  `event_visitors(event_id, day, visitor_hash)` provides exact daily uniques; both
  cascade when the event is purged.
- The gallery records `gallery_views` on every successful
  `GET /public/events/{slug}` (public or unlocked), `qr_scans` when `?src=qr` is
  present, and `downloads` when an `original` URL is issued. Recording is
  best-effort and never fails a gallery read.
- Visitor hashes are HMAC-SHA256 of `IP + UA` keyed by `ANALYTICS_HASH_SALT`
  (defaults to `JWT_SECRET`); no PII is stored and the same visitor on a new UTC
  day counts again.
- `GET /api/v1/events/{eventID}/analytics` and `GET /api/v1/account/analytics`
  return all-time totals plus a `?days=1..365` (default 30) UTC daily series, 404
  for other tenants, and aggregate the account across non-deleted events.
- Tests: unit (window validation, ownership, hashing, handler envelopes),
  integration (per-day upsert/unique math, tenant scoping, migration round trip),
  E2E (public view + QR + download → event/account analytics, cross-tenant 404).

### Slice 9D details

- `users.is_admin BOOLEAN NOT NULL DEFAULT FALSE` plus a partial `idx_users_is_admin`;
  there is no API to change it (operators are promoted with SQL).
- `internal/admin` owns the admin concern: `Repository` (cross-tenant SQL), `Service`
  (cursor/limit normalization, `degraded` health), `Handler` (envelopes), and
  `RequireAdmin` middleware mounted after `RequireAuth` (401 without auth, 403
  `FORBIDDEN` for non-admins, 500 for lookup failures).
- `GET /api/v1/admin/stats` returns users, non-deleted events, non-FAILED photos,
  summed tenant storage, paid-invoice revenue, and non-terminal subscriptions.
  `GET /api/v1/admin/users` and `/admin/subscriptions` are keyset paginated
  (`(created_at, id)`, default 20, max 100). `GET /api/v1/admin/health` reports pending/
  running/failed job counts and the oldest pending `run_at`.
- `storage.reconcile` runs daily (and at worker startup) and compares
  `SUM(users.storage_bytes)` and the non-`UPLOADING` photo count with the size/count of
  R2 keys under `tenant/` containing `/originals/` (derivatives, exports, and branding
  assets are excluded). Drift is reported through `admin.ReconcileReporter`
  (log-based in the worker); the job never deletes objects.
- `pkg/r2.S3Store.ListObjects(prefix)` follows `ListObjectsV2` pagination and is
  intentionally outside `r2.ObjectStore`; the reconcile job consumes a narrow
  `admin.ObjectLister` interface.
- Tests: unit (cursor/limit, service aggregates, `RequireAdmin` 200/401/403/500,
  handler envelopes, reconcile drift math), integration (repo aggregates and
  exclusions, queue health, `is_admin`, migration round trip, MinIO `ListObjects`),
  E2E (non-admin 403, stats/pagination/subscriptions/health, injected orphan drift
  detected and not deleted).
