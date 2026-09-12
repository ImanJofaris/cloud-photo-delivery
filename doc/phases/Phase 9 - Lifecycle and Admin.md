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
