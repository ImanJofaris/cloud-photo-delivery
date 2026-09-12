# Phase 5 — Public Gallery & Delivery

> **Status: COMPLETE.** See the [Phase 5 Report](Phase 5 - Report.md) for what was built and how it was verified.

**Goal:** Guests can open a public event by slug, browse photos with cursor pagination and lazy loading, view full screen, and download via short-lived signed URLs. This is the first customer-facing surface.

**Depends on:** Phase 4.

**Exit criteria:** A guest with no account can load an event, page through photos, open a photo, and download it; private/password events are protected; originals are never permanently public.

**Scope note (API-first):** Phase 5 delivers the **API only**. The Next.js gallery UI (§5) is deferred to a later phase; the API remains the source of truth.

**Locked decisions (see §8):**
- Unlock tokens are **stateless, event-scoped signed tokens** (separate issuer), not DB-backed.
- All image URLs are **short-lived signed URLs** (`SIGNED_URL_TTL`, default 5 min).
- Photo list/metadata responses **do not embed image URLs** (Option A); clients fetch URLs per variant from the `/url` endpoint.

---

## 1. Features

- Public event endpoint by slug (no auth).
- Cursor-paginated photo listing (50/page default), `READY` photos only.
- Photo item DTO with dimensions and derivative availability (no image URLs; see §4).
- Signed URL endpoint for thumbnail/medium/large and (if allowed) original download.
- Visibility handling: `public | password | private`.
  - `private` → not accessible publicly.
  - `password` → requires a session unlock token issued after password check.
- Download permissions from `event_settings`.
- Password-protected gallery unlock endpoint.
- All image URLs are short-lived signed URLs; no public bucket / permanent CDN URLs.
- Caching headers: metadata cacheable per visibility; signed-URL responses never cached.

---

## 2. API Surface

```http
GET  /api/v1/public/events/{slug}
GET  /api/v1/public/events/{slug}/photos?cursor=&limit=
GET  /api/v1/public/events/{slug}/photos/{photoID}
GET  /api/v1/public/events/{slug}/photos/{photoID}/url?variant=thumbnail|medium|large|original
POST /api/v1/public/events/{slug}/unlock          # password gallery
```

List response (metadata only — **no image URLs**, Option A):

```json
{
  "data": {
    "event": { "name": "Sarah & John's Wedding", "date": "2026-09-12" },
    "photos": {
      "items": [
        {
          "id": "uuid",
          "width": 2000,
          "height": 1333,
          "variants": ["thumbnail", "medium", "large"]
        }
      ],
      "nextCursor": "eyJpZCI6..."
    }
  },
  "error": null
}
```

Signed URL response (`GET .../photos/{photoID}/url?variant=large`):

```json
{
  "data": { "url": "https://.../optimized/{photoID}.webp?X-Amz-...", "expiresIn": 300 },
  "error": null
}
```

Clients fetch a URL per photo they actually render (fits lazy loading / infinite scroll); the metadata list stays cacheable because it contains no TTL-bound URLs.

---

## 3. Domain Layout

```text
internal/gallery/
  service.go        # visibility rules, unlock, permission checks
  repository.go     # cursor-paginated READY photo queries
  handler.go
  cursor.go
  tokens.go         # event-scoped unlock token issue/verify (separate issuer)
internal/photos/
  urls.go           # signed URL generation, variant selection
```

Cursor is an opaque base64 of `(created_at, id)` for stable keyset pagination.

---

## 4. Business Rules

- Only `status = READY` photos are listed.
- Cursor pagination only; `limit` capped (e.g. max 100).
- `allow_download = false` → no download URL endpoint; return `DOWNLOAD_DISABLED`.
- `allow_original_download = false` → `variant=original` returns `ORIGINAL_DOWNLOAD_DISABLED`.
- Signed URLs TTL = `SIGNED_URL_TTL` (default 5 min); never stored or logged.
- `variant` is limited to `thumbnail|medium|large|original`; unknown variant → `VALIDATION_ERROR`.
- Metadata responses never embed image URLs; URLs are fetched per variant from the `/url` endpoint.
- Password unlock issues a short-lived (`GALLERY_UNLOCK_TTL`, default 30 min), event-scoped token; not the account JWT.
- Unlock tokens use a **separate issuer** and carry an `event_id` claim; the gallery verifier rejects any token without a matching `event_id`, so an account access token cannot be replayed as an unlock token (and vice versa).
- Changing an event's password sets `event_settings.password_changed_at`; unlock tokens issued before that timestamp are rejected (bulk revocation without a token table).
- Private events return `EVENT_NOT_FOUND` publicly (don't reveal existence).
- Soft-deleted or expired events are inaccessible.
- Caching:
  - Public event metadata: `Cache-Control: public, max-age=60`.
  - Password event metadata and all `/url` responses: `Cache-Control: private, no-store` (responses depend on the unlock token or carry credentials in the URL).

---

## 5. Frontend (deferred)

The Next.js gallery UI is **out of scope for Phase 5**; this phase ships the API only. The planned UI (for a later phase) is:

```text
apps/web (Next.js)
  /e/[slug]              gallery page
  components: PhotoGrid, PhotoViewer, DownloadButton, UnlockForm
  data: fetch public API only, server components where possible
  images: fetch a signed URL per visible photo, lazy loading, IntersectionObserver
```

Keep UI minimal; API remains the source of truth.

---

## 6. Test Plan

**Unit**
- [x] cursor encode/decode round-trip; invalid cursor rejected.
- [x] visibility: private hidden; password requires valid token; expired token rejected.
- [x] unlock token with wrong/missing `event_id` claim rejected; account access token rejected as an unlock token.
- [x] unlock token issued before `password_changed_at` rejected.
- [x] download flags gate `variant=original` and `allow_download`.
- [x] variant selection maps to correct key; unknown variant rejected.
- [x] signed URL generated with correct key + TTL; original not returned when disabled.
- [x] list service excludes non-`READY` and soft-deleted; response contains no image URLs.
- [x] handler envelope/status and cache headers for all routes.

**Integration**
- [x] keyset pagination returns all photos exactly once, stable across equal timestamps.
- [x] repository scopes by event + status.
- [x] signed URL against MinIO actually retrieves the object.
- [x] password unlock token validated against the real settings row; revocation after password change.

**E2E**
- [x] Guest: open public event → paginate → open photo → get large URL → download works.
- [x] Password event: blocked without unlock, allowed after correct password, rejected after wrong.
- [x] `allow_original_download=false` blocks original, allows large.

---

## 7. Definition of Done

- [x] All master DoD items.
- [x] OpenAPI documents public endpoints (marked no-auth), the `url` variant enum, and no embedded image URLs in metadata.
- [x] `SIGNED_URL_TTL` and `GALLERY_UNLOCK_TTL` added to config with sensible defaults.
- [x] Migration adds `event_settings.password_changed_at` (nullable), reversible.
- [ ] Load smoke (Phase 10 refines): 1,000 concurrent metadata reads handled by CDN + cache.

---

## 8. Locked Decisions (rationale)

| Decision | Choice | Rationale |
|---|---|---|
| Unlock token | Stateless, event-scoped signed token (separate issuer, `event_id` claim) | No per-request DB read; scales to 1,000 concurrent viewers; reuses the existing `jwt/v5` + HS256 pattern in `internal/auth/tokens.go`. A separate issuer prevents cross-use with account access tokens. |
| Bulk revocation | `event_settings.password_changed_at` timestamp; reject earlier tokens | Gives "revoke all guest sessions on password change" without a token table. |
| Image URLs | Short-lived signed URLs for all variants | No public bucket / permanent CDN URL scheme needed; a leaked derivative URL expires in 5 min; keeps `password`/`private` events safe. |
| Metadata DTO | No embedded image URLs (Option A) | Keeps metadata responses cacheable (TTL-bound URLs would defeat CDN caching); client fetches URLs only for photos actually rendered. |
| `original` URL | Always signed and gated by `allow_original_download` | Originals are never permanently public. |
| Cache headers | `public, max-age=60` (public metadata); `private, no-store` (password metadata + all `/url`) | CDN absorbs public read load; credential-bearing/authorization-dependent responses are never cached. |

**Future path (deferred to Phase 8/10):** if egress/load demands it, add an optional `R2_PUBLIC_URL` and a `PublicURL(key)` method to `ObjectStore`, and return public URLs **only** for the `thumbnail` variant of `public` events. Keep this behind the `internal/photos/urls.go` interface so handlers are untouched.
