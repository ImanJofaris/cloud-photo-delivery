# Phase 5 — Public Gallery & Delivery — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Author:** Engineering

---

## 1. Summary

Phase 5 delivers the first customer-facing surface: a no-auth public gallery API. Guests open an event by slug, page through `READY` photos with keyset cursor pagination, inspect individual photos, and receive short-lived presigned URLs for thumbnails/medium/large and (when permitted) the original. Visibility is fully enforced — `private` events are indistinguishable from missing ones, and `password` events require a stateless, event-scoped unlock token. Metadata responses are CDN-cacheable while credential-bearing URL responses are never cached. The Next.js UI was intentionally deferred; this phase ships the API only.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| A guest with no account can load an event by slug | PASS | `TestE2E_PublicGalleryFlow`: `GET /api/v1/public/events/{slug}` → 200 with `Cache-Control: public, max-age=60`; all gallery routes registered outside the auth middleware in `cmd/api/main.go` |
| A guest can page through photos | PASS | `TestService_ListPhotos_Pagination` (5 photos, 3 pages, no duplicates); `TestRepository_ListReadyPhotos_KeysetStable` with equal timestamps returns all photos exactly once |
| A guest can open a photo | PASS | `TestHandler_GetPhoto_MetadataOnly`; `TestE2E_PublicGalleryFlow` `GET .../photos/{photoID}` → 200 |
| A guest can download via signed URL | PASS | `TestE2E_PublicGalleryFlow` gets `.../url?variant=large`, then HTTP-fetches the URL and receives the derivative bytes; `TestSignedURL_RetrievesObjectFromMinIO` verifies against real MinIO |
| Private events are protected | PASS | `TestService_GetEvent_PrivateHidden`, `TestHandler_GetEvent_PrivateReturns404`; `EventBySlug` excludes soft-deleted/expired (`TestRepository_EventBySlug_ExcludesDeletedAndExpired`) |
| Password events require unlock | PASS | `TestE2E_PasswordGalleryFlow`: list without token → 401, wrong password → 401, correct password → token, list with token → 200 |
| Originals are never permanently public | PASS | All variants use `PresignGet` with `SIGNED_URL_TTL` (5m); original gated by `allow_original_download` (`TestService_PhotoURL_OriginalDisabled`, E2E `ORIGINAL_DOWNLOAD_DISABLED`) |
| Revocation on password change | PASS | `TestUnlockTokens_NotBeforeRevokesOlderTokens`, `TestService_Unlock_RevokedAfterPasswordChange`, `TestSettings_PasswordSetsChangedAt` |

---

## 3. What was built

### 3.1 Gallery package (`internal/gallery/`)

| File | Responsibility |
|---|---|
| `model.go` | `VisibleEvent`, `PhotoPage`, `Variant` (with validation/original check), `VariantsFor`, and the read-only `Repository` interface. |
| `cursor.go` | Opaque base64 keyset cursor over `(created_at, id)`; `DecodeCursor` rejects malformed input; `NormalizeLimit` caps at 100 (default 50). |
| `tokens.go` | `UnlockTokens`: HS256 JWT issue/verify with a **gallery-specific issuer** (`cloud-photo-delivery-gallery`) and an `event_id` claim; verifies event scope and rejects tokens issued before `password_changed_at`. |
| `repository.go` | Read-only SQL: `EventBySlug` (excludes deleted/expired), `GetSettingsByEventID`, `ListReadyPhotos` (keyset, `READY` only), `ReadyPhoto` (event + status scoped). |
| `service.go` | Visibility rules (`private` → 404, `password` → token, else unauthorized), unlock, listing/pagination, photo fetch, and signed-URL gating (`DOWNLOAD_DISABLED`, `ORIGINAL_DOWNLOAD_DISABLED`, `VALIDATION_ERROR`, `VARIANT_UNAVAILABLE`). |
| `handler.go` | HTTP layer only: DTOs, `X-Gallery-Unlock` header, limits/cursor parsing, envelope responses, and `Cache-Control` (`public, max-age=60` vs `private, no-store`). |

### 3.2 Signed URLs (`internal/photos/urls.go`)

- `SignedURLGenerator` wraps a `PresignGet` interface (satisfied by `pkg/r2.S3Store`) and maps `thumbnail|medium|large|original` to the corresponding storage key.
- Returns `{url, expiresIn}`; unknown or missing variants return `ErrVariantUnavailable`, mapped to a 404 by the gallery service.
- Isolated so a future public-CDN path (Phase 8/10) can replace it without touching handlers.

### 3.3 Password-change revocation (`internal/events/`)

- `events.Settings` gained `PasswordChangedAt *time.Time`; the repository selects and writes `password_changed_at`, and `buildSettings` stamps it (truncated to seconds) whenever a password is set, changed, or cleared.
- Unlock tokens issued before that timestamp are rejected, giving bulk guest-session revocation without a token table.

### 3.4 Wiring

- `cmd/api/main.go` builds the gallery repository/service/handler and mounts `/api/v1/public/events/{slug}` **outside** `RequireAuth`.
- `internal/platform/config` adds `SIGNED_URL_TTL` (5m) and `GALLERY_UNLOCK_TTL` (30m); `.env.example` documents both.
- `auth.VerifyPassword` (bcrypt) is injected into the gallery service as the password verifier.

---

## 4. Database changes

```text
migrations/0006_gallery.sql
```

- `ALTER TABLE event_settings ADD COLUMN password_changed_at TIMESTAMPTZ` (nullable; no backfill needed).
- `CREATE INDEX idx_photos_event_status_created ON photos(event_id, status, created_at DESC, id DESC)` to serve the keyset listing.
- Rollback drops the index and column; covered by `TestMigrations_GalleryUpDownRoundTrip`.

---

## 5. API surface added

```http
GET  /api/v1/public/events/{slug}
GET  /api/v1/public/events/{slug}/photos?cursor=&limit=
GET  /api/v1/public/events/{slug}/photos/{photoID}
GET  /api/v1/public/events/{slug}/photos/{photoID}/url?variant=thumbnail|medium|large|original
POST /api/v1/public/events/{slug}/unlock
```

All documented in `api/openapi.yaml` (v0.5.0) with `security: []`, the variant enum, cache headers, and the metadata-only contract. Password events read the `X-Gallery-Unlock` header.

List response (metadata only — no image URLs):

```json
{
  "data": {
    "event": { "name": "Sarah & John's Wedding", "date": "2026-09-12", "requiresUnlock": false },
    "photos": {
      "items": [{ "id": "…", "width": 2000, "height": 1333, "variants": ["thumbnail", "medium", "large"] }],
      "nextCursor": "eyJpZCI6…"
    }
  },
  "error": null
}
```

Signed URL response:

```json
{ "data": { "url": "https://…/optimized/{photoID}.webp?X-Amz-…", "expiresIn": 300 }, "error": null }
```

---

## 6. Files created / modified

```text
migrations/0006_gallery.sql                                  (new)
migrations/migrations_integration_test.go                    (modified: 0006 round-trip)

internal/gallery/model.go                                    (new)
internal/gallery/cursor.go                                   (new)
internal/gallery/tokens.go                                   (new)
internal/gallery/repository.go                               (new)
internal/gallery/service.go                                  (new)
internal/gallery/handler.go                                  (new)
internal/gallery/fakes_test.go                               (new)
internal/gallery/cursor_test.go                              (new)
internal/gallery/tokens_test.go                              (new)
internal/gallery/service_test.go                             (new)
internal/gallery/handler_test.go                             (new)
internal/gallery/repository_integration_test.go              (new)

internal/photos/urls.go                                      (new)
internal/photos/urls_test.go                                 (new)

internal/events/model.go                                     (modified: PasswordChangedAt)
internal/events/repository.go                                (modified: select/write password_changed_at)
internal/events/service.go                                   (modified: stamp on password change)
internal/events/service_test.go                              (modified: changed-at tests)
internal/events/repository_integration_test.go               (modified: schema)

internal/platform/config/config.go                           (modified: TTLs)
internal/platform/config/config_test.go                      (modified: defaults)

cmd/api/main.go                                              (modified: gallery wiring + public routes)
api/openapi.yaml                                             (modified: v0.5.0 public endpoints)
.env.example                                                 (modified: SIGNED_URL_TTL, GALLERY_UNLOCK_TTL)

test/e2e/gallery_flow_test.go                                (new)
test/e2e/events_flow_test.go                                 (modified: schema)
test/e2e/uploads_flow_test.go                                (modified: schema)
```

---

## 7. Tests

### Unit
- **cursor:** round-trip, empty → nil, malformed base64/timestamp/uuid rejected, limit normalization.
- **tokens:** issue/verify, wrong event rejected, wrong secret rejected, expiry rejected, account-issuer token rejected, `notBefore` revocation.
- **service:** public access, private hidden, password requires unlock, unlock success/wrong/empty/not-protected/revoked, pagination across three pages with no duplicates and gaps, invalid cursor, download disabled, original disabled, original uses storage key, invalid variant, non-`READY`/missing photo, `VariantsFor`.
- **handler:** public vs password cache headers, private 404, unlock success/no-store/invalid JSON, metadata-only list, 401 without token, photo metadata, URL cache header + `expiresIn`, invalid variant 422.
- **photos/urls:** variant→key mapping for all four variants, invalid variant, missing derivative, store error propagation.
- **events:** `password_changed_at` set on set/clear, unchanged when editing unrelated settings.

### Integration (`//go:build integration`)
- **gallery repository:** slug lookup, deleted/expired exclusion, keyset pagination with equal timestamps (exactly once), event+status scoping, settings read.
- **signed URL vs MinIO:** presigned GET generated and fetched successfully.
- **migrations:** `0006` up/down round-trip (column + index).
- **E2E public flow:** signup → event → seeded `READY` photo + MinIO object → guest event/photo/list → signed large URL → HTTP download of derivative bytes → original blocked.
- **E2E password flow:** signup → event → settings PATCH with password → blocked without token → wrong password 401 → correct password token → list with token 200 and `private, no-store`.

### Verification commands and results

```text
gofmt -l .                                     -> clean
go build ./...                                 -> OK
go vet ./...                                   -> OK
go vet -tags=integration ./...                 -> OK
go test ./...                                  -> all packages PASS
go test -tags=integration -p 1 ./...           -> all packages PASS
```

Package coverage (`internal/gallery`, integration included): **83.8%** total; `cursor.go` 93–100%, `tokens.go` 87–100%, `repository.go` 86–100%, `service.go` 75–100%, `handler.go` 50–100% (floor 80% met at package level).

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| E2E password flow returned `INTERNAL_ERROR` on settings PATCH | `GetSettings` still selected 7 columns after `scanSettings` was widened for `password_changed_at`; updated the joined select list. |
| Unlock token issued in the same second as the password change was wrongly revoked | JWT `iat` is second-precision while `password_changed_at` was microseconds; truncate the revocation timestamp to `time.Second` (UTC) on write. |
| Existing inline test schemas lacked `password_changed_at` | Added the column to the events/uploads E2E schemas and the events repository integration schema. |
| First `TestE2E_PublicGalleryFlow` run failed waiting for MinIO | Container start timed out once (101s); a clean rerun passed in 13s — environment flake, not a code defect. |

---

## 9. Known limitations / follow-ups

- **Frontend deferred.** No `apps/web`; the Next.js gallery (grid, lazy loading, viewer) is a later phase.
- **Load smoke deferred.** The 1,000 concurrent metadata read check remains a Phase 10 item; metadata responses are already CDN-friendly (`public, max-age=60`).
- **Public endpoints are not rate-limited.** `main.go` still applies `authLimiter` only to auth routes; hardening is Phase 10.
- **View-only galleries are not possible.** Per the phase spec, `allow_download=false` returns `DOWNLOAD_DISABLED` for every variant, so guests cannot render images either. A view/download distinction (or watermark-only viewing) should be addressed in Phase 7 (branding/watermark).
- **`watermark_enabled` is still unused** by the read path; derivates are served unmodified.
- **No per-event CDN caching or public bucket** by design; the `internal/photos/urls.go` seam allows a later `PublicURL` path for public-event thumbnails.
- `-race` not run locally (no gcc); CI runs it on Linux.

---

## 10. How to try it

```text
copy .env.example .env
make up; make migrate-up
make run                                  # API (HTTP_ADDR=:18080 if 8080 is reserved)
make worker                               # second terminal

# signup, create an event, upload + complete a JPEG (see Phase 3/4 reports),
# wait for READY, then as an anonymous guest:

curl.exe http://localhost:18080/api/v1/public/events/<slug>

curl.exe "http://localhost:18080/api/v1/public/events/<slug>/photos?limit=50"

curl.exe "http://localhost:18080/api/v1/public/events/<slug>/photos/<photoId>/url?variant=large"
# open the returned url in a browser (valid for 5 minutes)

# password gallery:
curl.exe -X PATCH http://localhost:18080/api/v1/events/<eventId>/settings ^
  -H "Authorization: Bearer <accessToken>" -H "Content-Type: application/json" ^
  --data-binary "@settings.json"          # {"visibility":"password","password":"open-sesame"}

curl.exe -X POST http://localhost:18080/api/v1/public/events/<slug>/unlock ^
  -H "Content-Type: application/json" --data-binary "@unlock.json"   # {"password":"open-sesame"}
# then send the token as:  X-Gallery-Unlock: <token>
```
