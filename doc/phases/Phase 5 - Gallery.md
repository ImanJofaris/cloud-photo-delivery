# Phase 5 — Public Gallery & Delivery

**Goal:** Guests can open a public event by slug, browse photos with cursor pagination and lazy loading, view full screen, and download via short-lived signed URLs. This is the first customer-facing surface.

**Depends on:** Phase 4.

**Exit criteria:** A guest with no account can load an event, page through photos, open a photo, and download it; private/password events are protected; originals are never permanently public.

---

## 1. Features

- Public event endpoint by slug (no auth).
- Cursor-paginated photo listing (50/page default), `READY` photos only.
- Photo item DTO with derivative keys/sizes and CDN URLs.
- Signed URL endpoint for full image and (if allowed) original download.
- Visibility handling: `public | password | private`.
  - `private` → not accessible publicly.
  - `password` → requires a session unlock token issued after password check.
- Download permissions from `event_settings`.
- Password-protected gallery unlock endpoint.
- Next.js public gallery page: cover, grid, lazy loading, infinite scroll, fullscreen viewer, download button.
- CDN-friendly caching headers on derivative request metadata (signed URLs bypass cache; metadata is cacheable per visibility).

---

## 2. API Surface

```http
GET  /api/v1/public/events/{slug}
GET  /api/v1/public/events/{slug}/photos?cursor=&limit=
GET  /api/v1/public/events/{slug}/photos/{photoID}
GET  /api/v1/public/events/{slug}/photos/{photoID}/url?variant=large|original
POST /api/v1/public/events/{slug}/unlock          # password gallery
```

List response:

```json
{
  "data": {
    "event": { "name": "Sarah & John's Wedding", "date": "2026-09-12", "cover": "..." },
    "photos": {
      "items": [
        {
          "id": "uuid",
          "width": 2000,
          "height": 1333,
          "thumbnailUrl": "https://cdn/.../thumb.webp",
          "largeUrl": "https://cdn/.../large.webp"
        }
      ],
      "nextCursor": "eyJpZCI6..." 
    }
  },
  "error": null
}
```

---

## 3. Domain Layout

```text
internal/gallery/
  service.go        # visibility rules, unlock, permission checks
  repository.go     # cursor-paginated READY photo queries
  handler.go
  cursor.go
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
- Signed URLs TTL ~5 min; never stored or logged.
- Password unlock issues a short-lived, event-scoped token; not the account JWT.
- Private events return `EVENT_NOT_FOUND` publicly (don't reveal existence).
- Soft-deleted or expired events are inaccessible.

---

## 5. Frontend (Phase 5 UI scope)

```text
apps/web (Next.js)
  /e/[slug]              gallery page
  components: PhotoGrid, PhotoViewer, DownloadButton, UnlockForm
  data: fetch public API only, server components where possible
  images: next/image with remote CDN loader, lazy loading, IntersectionObserver
```

Keep UI minimal; API remains the source of truth.

---

## 6. Test Plan

**Unit**
- [ ] cursor encode/decode round-trip; invalid cursor rejected.
- [ ] visibility: private hidden; password requires valid token; expired token rejected.
- [ ] download flags gate `variant=original` and `allow_download`.
- [ ] signed URL generated with correct key + TTL; original not returned when disabled.
- [ ] list service excludes non-`READY` and soft-deleted.
- [ ] handler envelope/status for all routes.

**Integration**
- [ ] keyset pagination returns all photos exactly once, stable across equal timestamps.
- [ ] repository scopes by event + status.
- [ ] signed URL against MinIO actually retrieves the object.
- [ ] password unlock token stored/validated correctly.

**E2E**
- [ ] Guest: open public event → paginate → open photo → get large URL → download works.
- [ ] Password event: blocked without unlock, allowed after correct password, rejected after wrong.
- [ ] `allow_original_download=false` blocks original, allows large.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents public endpoints (marked no-auth).
- [ ] Load smoke (Phase 10 refines): 1,000 concurrent metadata reads handled by CDN + cache.
