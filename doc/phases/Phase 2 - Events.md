# Phase 2 — Events & Settings

**Goal:** Operators can create, list, read, update, archive, and delete their own events, with per-event gallery settings. This is the tenant boundary for all photo data.

**Depends on:** Phase 1.

**Exit criteria:** Event CRUD fully tenant-scoped; cross-tenant access returns 404; settings CRUD works; slug uniqueness per tenant enforced.

---

## 1. Features

- Create event: name, event date, client name, client email, location, description.
- Auto-generate URL slug from name, unique per tenant.
- Event status lifecycle: `upcoming | active | completed | archived`.
- List events with filter (status) + cursor pagination + search by name.
- Get / update / archive event.
- Soft delete (Phase 9 hardens deletion and grace period).
- Event dashboard aggregates: photo count, storage bytes, guest count (stub counts now, filled in later phases).
- Per-event settings: visibility, download enabled, original download enabled, password protection, watermark, expiry.
- Ownership enforced in repository (`WHERE id = $1 AND user_id = $2`).

---

## 2. Database (migration `0003_events.sql`)

```sql
CREATE TABLE events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    client_name VARCHAR(255),
    client_email VARCHAR(320),
    location VARCHAR(255),
    description TEXT,
    event_date DATE,
    status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
    cover_photo_id UUID,
    storage_bytes BIGINT NOT NULL DEFAULT 0,
    photo_count BIGINT NOT NULL DEFAULT 0,
    guest_count BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, slug)
);

CREATE TABLE event_settings (
    event_id UUID PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    visibility VARCHAR(30) NOT NULL DEFAULT 'public',   -- public|password|private
    password_hash TEXT,
    allow_download BOOLEAN NOT NULL DEFAULT TRUE,
    allow_original_download BOOLEAN NOT NULL DEFAULT FALSE,
    watermark_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_user_status ON events(user_id, status);
CREATE INDEX idx_events_user_created ON events(user_id, created_at DESC);
```

---

## 3. API Surface

```http
POST   /api/v1/events
GET    /api/v1/events?status=&cursor=&limit=&q=
GET    /api/v1/events/{eventID}
PATCH  /api/v1/events/{eventID}
POST   /api/v1/events/{eventID}/archive
DELETE /api/v1/events/{eventID}

GET    /api/v1/events/{eventID}/settings
PATCH  /api/v1/events/{eventID}/settings
GET    /api/v1/events/{eventID}/dashboard
```

Status transition rules (validated in service):

```text
upcoming  -> active | archived
active    -> completed | archived
completed -> archived
archived  -> (terminal)
```

---

## 4. Domain Layout

```text
internal/events/
  service.go        # rules, slug generation, status transitions, limits
  repository.go      # tenant-scoped SQL
  handler.go
  settings.go
  cursor.go          # encode/decode (id + created_at)
```

---

## 5. Business Rules

- Slug: lowercase, hyphenated, ASCII; collisions get `-2`, `-3`.
- Default status `upcoming`; can set `active` when event begins.
- Free-plan active-event limit enforced here (limit value from Phase 8; default 1 until then via a `PlanLimits` interface).
- Deleting sets `deleted_at` (soft) and excludes from lists; hard delete deferred to Phase 9 worker.
- `expires_at` derived from retention at creation (Phase 8/9); nullable = never.

---

## 6. Test Plan

**Unit**
- [ ] slug: normalization, uniqueness suffixing, empty/invalid names.
- [ ] status transitions: valid accepted, invalid rejected.
- [ ] create: client email validation, required name.
- [ ] active-event limit reached → `PLAN_LIMIT_REACHED`.
- [ ] get/update/delete on missing or other-tenant event → `EVENT_NOT_FOUND`.
- [ ] settings: enabling password requires password; private hides downloads.
- [ ] cursor encode/decode round-trip; malformed cursor rejected.
- [ ] handler envelope/status for each route.

**Integration**
- [ ] repository CRUD against Postgres; `UNIQUE(user_id, slug)` violation handled.
- [ ] tenant isolation: user B cannot read/update/delete user A's event (0 rows).
- [ ] soft-deleted events excluded from list and get.
- [ ] settings row created transactionally with event.
- [ ] pagination: stable ordering with equal `created_at`, no duplicates/skips.

**E2E**
- [ ] signup → create 3 events → list with filter → update one → archive → delete → list excludes it.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI updated for events + settings.
- [ ] Tenant isolation test present and passing.
