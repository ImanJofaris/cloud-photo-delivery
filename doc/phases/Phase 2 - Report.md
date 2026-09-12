# Phase 2 — Events & Settings — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Author:** Engineering

---

## 1. Summary

Phase 2 delivered the tenant boundary for all photo data: full event CRUD with auto-generated per-tenant slugs, a validated status lifecycle, cursor-based list/search, soft delete, per-event gallery settings (visibility, password, downloads, watermark), and dashboard aggregates. Every event query is scoped by `user_id` in the repository, so one operator can never see or mutate another's events. The events domain is fully unit-testable (no DB, no HTTP), backed by repository integration tests and an end-to-end HTTP flow.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Event CRUD fully tenant-scoped | PASS | Handler routes under `RequireAuth`; repo queries use `WHERE id = $1 AND user_id = $2`; live smoke create/list/get/update/delete all 2xx |
| Cross-tenant access returns 404 | PASS | Live: user B reading/deleting user A's event -> `404 EVENT_NOT_FOUND`; `TestE2E_EventsTenantIsolation`, `TestRepository_TenantIsolation` |
| Settings CRUD works | PASS | Live `PATCH/GET /events/{id}/settings` returns updated visibility/password/downloads; unit + integration tests |
| Slug uniqueness per tenant enforced | PASS | `CONSTRAINT events_slug_unique UNIQUE(user_id, slug)`; `TestRepository_UniqueSlugPerTenant`; service collision suffixing unit tests |
| `make test` (unit) passes | PASS | `go test ./...` all packages ok |
| Integration tests pass | PASS | `go test -tags=integration ./...` all packages ok |
| gofmt / build / vet clean | PASS | `gofmt -l .` clean; `go build`, `go vet`, `go vet -tags=integration` clean |
| No secrets logged | PASS | No passwords/hashes logged; event password stored bcrypt-hashed and never returned |
| OpenAPI documented | PASS | `api/openapi.yaml` v0.3.0 with all event + settings paths and schemas |

---

## 3. What was built

### 3.1 Events domain (`internal/events/`)

| File | Responsibility |
|---|---|
| `model.go` | `Event`, `Settings`, `Status`, `Visibility`, transition table and helpers. |
| `slug.go` | ASCII lowercase hyphenation, numeric collision suffixing, length cap. |
| `cursor.go` | Opaque base64 keyset cursor over `(created_at, id)`; limit normalization. |
| `repository.go` | All tenant-scoped SQL: CRUD, list/filter/search/pagination, status, soft delete, settings, transactional create. |
| `service.go` | Business rules: validation, slug uniqueness, status transitions, active-event limit, settings rules, password hashing. |
| `handler.go` | HTTP parsing, DTO shaping, standard envelope, status codes. |

### 3.2 Plan limits boundary (`internal/platform/limits/`)

| File | Responsibility |
|---|---|
| `limits.go` | `PlanLimits` interface with `MaxActiveEvents`; `Default` implementation allows unlimited. Injected into the events service so Phase 8 billing can replace it without an import cycle. |

### 3.3 Wiring

| File | Responsibility |
|---|---|
| `cmd/api/main.go` | Constructs events repository/service/handler; mounts event routes under the existing `RequireAuth` group. |

---

## 4. Database changes

```text
migrations/0003_events.sql
```

- `events` — id, user_id (FK users, cascade), name, slug, client_name, client_email, location, description, event_date, status, cover_photo_id, storage_bytes, photo_count, guest_count, expires_at, deleted_at, created_at, updated_at.
  - `CONSTRAINT events_slug_unique UNIQUE (user_id, slug)`.
  - `CHECK (status IN ('upcoming','active','completed','archived'))`.
- `event_settings` — event_id (PK, FK events cascade), visibility, password_hash, allow_download, allow_original_download, watermark_enabled, updated_at.
  - `CHECK (visibility IN ('public','password','private'))`.
- Indexes: `idx_events_user_status`, `idx_events_user_created` (`created_at DESC, id DESC`), `idx_events_user_deleted`.

`event_settings` is inserted in the same transaction as the event, so an event can never exist without settings. Rollback (`-- +goose Down`) drops both tables; covered by `TestMigrations_EventsUpDownRoundTrip`.

**Deviation from the phase doc:** the migration uses `DEFAULT gen_random_uuid()` on the id and adds a `CHECK` constraint on status/visibility, and `user_id REFERENCES users(id) ON DELETE CASCADE`. These are defensive additions consistent with `0002_auth.sql`; they do not change the documented schema semantics.

---

## 5. API surface added

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

Create response:

```json
{
  "data": {
    "event": {
      "id": "15eba470-0bde-4c83-a783-b23a40c88eba",
      "name": "Smoke Test Wedding",
      "slug": "smoke-test-wedding",
      "clientName": "Aisyah",
      "clientEmail": "aisyah@example.com",
      "location": "Kuala Lumpur",
      "description": "Live smoke event",
      "eventDate": "2026-10-01",
      "status": "upcoming",
      "coverPhotoId": null,
      "storageBytes": 0,
      "photoCount": 0,
      "guestCount": 0,
      "expiresAt": null,
      "createdAt": "2026-09-12T09:04:39Z",
      "updatedAt": "2026-09-12T09:04:39Z"
    },
    "settings": {
      "eventId": "15eba470-0bde-4c83-a783-b23a40c88eba",
      "visibility": "public",
      "passwordProtected": false,
      "allowDownload": true,
      "allowOriginalDownload": false,
      "watermarkEnabled": false,
      "updatedAt": "2026-09-12T09:04:39Z"
    }
  },
  "error": null
}
```

List envelope:

```json
{ "data": { "items": [ ... ], "nextCursor": "MjAyNi0wOS0xMlQwOTowNjo..." }, "error": null }
```

`DELETE` returns `204 No Content`. Error codes used: `EVENT_NOT_FOUND` (404), `VALIDATION_ERROR` (422), `INVALID_STATUS_TRANSITION` (409), `PLAN_LIMIT_REACHED` (402).

---

## 6. Files created / modified

```text
migrations/0003_events.sql
migrations/migrations_integration_test.go            (modified)

internal/platform/limits/limits.go

internal/events/model.go
internal/events/slug.go
internal/events/cursor.go
internal/events/repository.go
internal/events/service.go
internal/events/handler.go
internal/events/slug_test.go
internal/events/cursor_test.go
internal/events/service_test.go
internal/events/handler_test.go
internal/events/repository_integration_test.go

test/e2e/events_flow_test.go

cmd/api/main.go                                      (modified)
api/openapi.yaml                                     (rewritten, v0.3.0)
```

---

## 7. Tests

### Unit
- slug: normalization (case, punctuation, apostrophe, non-ASCII, empty), collision suffixing, length cap.
- cursor: encode/decode round-trip, empty, malformed, base64, limit normalization.
- service: create validation (name required, invalid client email, invalid status), default status/visibility/download, slug generation + uniqueness within a tenant, same slug across tenants, active-event limit on create, invalid status, get/update/delete not-found for other tenant, status transitions valid/invalid/terminal, archive, settings password rules, private disables downloads, original-requires-download, list invalid status/cursor.
- handler: create 201, validation 422, invalid JSON 422, list 200 + items, list invalid status 422, get 200/404 `EVENT_NOT_FOUND`, update 200, archive 200, delete 204 empty body, get/patch settings, dashboard, unauthenticated 401.

### Integration (`//go:build integration`)
- repository create + settings created transactionally; defaults.
- `UNIQUE(user_id, slug)` violation on duplicate; same slug allowed across tenants.
- tenant isolation: user B cannot get/update/delete user A's event.
- soft-deleted events excluded from get and list; second delete is not-found.
- update fields incl. clearing event date.
- `SetStatus` + `CountByStatus`.
- list status filter, name search, and stable keyset pagination with equal `created_at` (no duplicates across pages).
- settings tenant scoping; cascade delete of settings when event removed.
- migration `0003` applies and rolls back cleanly.

### E2E
- signup -> create 3 events -> list all -> filter `status=upcoming` -> update one -> archive one -> filter `status=archived` -> update settings (password) -> delete -> list excludes deleted.
- tenant isolation: user B gets 404 on user A's event for get/update/delete.

### Verification commands and results

```text
gofmt -l .                          -> clean
go build ./...                      -> OK
go vet ./...                        -> OK
go vet -tags=integration ./...      -> OK
go test ./...                       -> all packages PASS
go test -tags=integration ./...     -> all packages PASS (incl. migrations + e2e)
```

Live smoke against Postgres + API on `:18080`: signup 201; create 201 (slug `smoke-test-wedding`); list/search/pagination 200; get 200; update 200; settings PATCH/GET 200; dashboard 200; archive 200; delete 204; post-delete get 404 and list empty; unauthenticated 401; cross-tenant get/delete 404.

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `UPDATE event_settings ... FROM events` raised `column reference "updated_at" is ambiguous` | Rewrote as `UPDATE event_settings ... WHERE EXISTS (SELECT 1 FROM events ...)`; no join ambiguity. |
| `GetSettings` join selected unqualified `updated_at`, ambiguous across `events`/`event_settings` | Qualified all selected settings columns with the `s.` alias. |
| `allowDownload` defaulted to `false` on create (zero-value struct bypassed the DB default) | Seed the create path with `Settings{Visibility: Public, AllowDownload: true}` before merging input; explicit `false` still sticks. |
| Malformed base64 cursor test vector included a space | Replaced with valid base64 strings that decode to invalid cursors. |

---

## 9. Known limitations / follow-ups

- Dashboard aggregates are real column counts but remain `0` until Phase 3/4 populate `photo_count`, `storage_bytes`, `guest_count`.
- `PlanLimits.Default` allows unlimited active events; Phase 8 injects the real plan-based implementation. The interface is already in place.
- Soft delete only sets `deleted_at`; hard-delete/grace period and admin purge are Phase 9.
- Slug changes on rename are intentionally not applied (slug is stable for share links); revisit if product requires editable slugs.
- `expires_at` is accepted/stored but not yet enforced or derived from retention (Phase 8/9).
- `-race` not run locally (no gcc); CI runs it on Linux.

---

## 10. How to try it

```text
copy .env.example .env
make up
make migrate-up
make run

# if port 8080 is blocked on Windows, set HTTP_ADDR=:18080 in .env

# sign up and capture accessToken
curl -X POST http://localhost:8080/api/v1/auth/signup \
  -H "Content-Type: application/json" \
  -d '{"email":"me@example.com","password":"password123","businessName":"My Booth"}'

# create an event
curl -X POST http://localhost:8080/api/v1/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <accessToken>" \
  -d '{"name":"Summer Party","eventDate":"2026-10-01","clientEmail":"client@example.com"}'

# list, get, update, settings
curl http://localhost:8080/api/v1/events?status=upcoming -H "Authorization: Bearer <accessToken>"
curl http://localhost:8080/api/v1/events/<eventID> -H "Authorization: Bearer <accessToken>"
curl -X PATCH http://localhost:8080/api/v1/events/<eventID> \
  -H "Content-Type: application/json" -H "Authorization: Bearer <accessToken>" \
  -d '{"name":"Summer Party 2026"}'
curl -X PATCH http://localhost:8080/api/v1/events/<eventID>/settings \
  -H "Content-Type: application/json" -H "Authorization: Bearer <accessToken>" \
  -d '{"visibility":"password","password":"guestpass"}'

# archive and delete
curl -X POST http://localhost:8080/api/v1/events/<eventID>/archive -H "Authorization: Bearer <accessToken>"
curl -X DELETE http://localhost:8080/api/v1/events/<eventID> -H "Authorization: Bearer <accessToken>"
```
