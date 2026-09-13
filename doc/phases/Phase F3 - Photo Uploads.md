# Phase F3 — Photo Uploads & Library

> **Status: COMPLETE.** See the [Phase F3 Report](Phase F3 - Report.md) for what was built and how it was verified.

**Goal:** Operators upload photo batches straight from the browser to R2, watch per-file progress, recover interrupted uploads after a reload, and manage the event library (processing status, failures, deletion).

**Depends on:** Phase F2; Phase 3 (uploads API); Phase 4 (processing worker); backend prerequisites in §2.

**Exit criteria:** An operator can drag a batch of JPEG/PNG/WebP files into an event, see per-file progress and retries, watch photos move `PROCESSING → READY` without reloading, resume an interrupted upload after a page reload, and delete a photo. Photo bytes flow browser ⇄ R2 only, and foreign ids surface as not-found.

---

## 1. Features

- Tabbed event detail (`Overview / Photos / Settings / Danger`); Photos tab hosts the uploader and library grid.
- Drag-and-drop and file picker (multi-select), client validation mirroring the API (jpeg/png/webp, 1 B–100 MB, non-empty filename).
- Presigned uploads: simple PUT `< 10 MB`, multipart `>= 10 MB`, behind one uploader interface.
- Per-file and batch progress via `XMLHttpRequest`; bounded concurrency (3); retry with exponential backoff; one failed file never blocks the batch.
- Idempotency keys on initialize and complete.
- Resume after reload: upload registry in `localStorage`, reconciled against `GET /uploads/{photoID}`.
- Cancel/abort in-flight uploads, including multipart abort.
- Photo grid: cursor paginated, newest first, status badges (`UPLOADING | PROCESSING | READY | FAILED`), polling until terminal, error reasons.
- Delete photo with confirmation; optimistic removal; object cleanup.
- Dashboard aggregates (`photo_count`, `storage_bytes`) invalidated on completion and delete.

---

## 2. Backend prerequisites (land first)

The API has no owner-facing photo list, no owner signed URLs, no re-presign, and no photo delete. Per the API-first rule these ship before the UI, in this order.

### 2.1 OpenAPI changes (`api/openapi.yaml`)

```http
GET    /api/v1/events/{eventID}/photos?cursor=&limit=&status=
GET    /api/v1/photos/{photoID}/url?variant=thumbnail|medium|large|original
POST   /api/v1/uploads/{photoID}/url
DELETE /api/v1/photos/{photoID}
```

- `GET /events/{eventID}/photos` — bearer auth; the event must belong to the caller (`EVENT_NOT_FOUND` otherwise). Cursor-paginated, newest first, optional `status` filter (`UPLOADING|PROCESSING|READY|FAILED`), default `limit 50` (max 100). Returns `PhotoList`:
  - `items[]`: `id`, `filename`, `mimeType`, `fileSize`, `width`, `height`, `status`, `variants` (derivatives that exist), `errorMessage` (sanitized), `createdAt`.
  - `nextCursor` (nullable).
  - New schemas `Photo` and `PhotoList`; do not reuse `PublicPhoto`.
- `GET /photos/{photoID}/url?variant=` — bearer auth; owner/device scoped. Returns the existing `SignedURL` shape (`url`, `expiresIn`). Errors: `PHOTO_NOT_FOUND` for foreign ids, `VARIANT_UNAVAILABLE` when the derivative does not exist, `VALIDATION_ERROR` for an unknown variant. Event download settings gate **guests**, not the owner: an operator may always fetch `original`. Response is `Cache-Control: private, no-store`.
- `POST /uploads/{photoID}/url` — bearer/device auth; re-issues a simple presigned PUT for a photo still `UPLOADING` with `uploadKind=simple`; returns `{ uploadUrl, expiresAt }`. `409 NOT_SIMPLE` for multipart, `409 INVALID_UPLOAD_STATE` for completed photos, `404 PHOTO_NOT_FOUND` otherwise.
- `DELETE /photos/{photoID}` — bearer auth; owner scoped; `204`. Deletes the row, aborts any in-progress multipart upload, enqueues object cleanup for every key, and decrements counters exactly once (see §2.2).
- Existing upload/status schemas stay as they are; regenerate `packages/api-client` (`pnpm gen:api`) and commit before UI work.

### 2.2 Go changes

- `internal/photos/repository.go`: owner-scoped `ListByEvent(ListInput)` with keyset pagination `(created_at, id)` — same opaque cursor format as events/gallery, implemented in `internal/photos/cursor.go`. `EventOwnedBy(userID, eventID)` for authorization and `DeleteOwned(photoID, userID)` returning `(*DeletedPhoto, error)`. The service authorizes via `GetByID` + `EventOwnedBy` (mirroring `internal/uploads`), and devices compare their assigned event directly.
- `internal/photos/handler.go` (new): owner-facing list/url/delete handlers, wired in `cmd/api` under the authenticated group; envelope + error mapping as everywhere else.
- `internal/uploads/service.go`: `RePresign(actor, photoID)` for simple uploads.
- `internal/photos/cleanup.go`: extend `CleanupPayload` with optional `keys []string`. When present, delete every key and no-op if the row is gone; keep the existing failed-processing behavior. Idempotent, ignores `r2.ErrNotFound`.
- Delete transaction: abort multipart (if any) → collect `storage_key` + derivative keys → delete row + `upload_parts` (cascade) and decrement `events.photo_count`/`storage_bytes` in one transaction → enqueue `object.cleanup` with the key snapshot.
  - Counters increment in `MarkProcessing` (`internal/photos/repository.go:126`), so decrement `storage_bytes` by `file_size` and `photo_count` by 1 **only when the photo status is not `UPLOADING`**. Never double-decrement on retried deletes.
- Tests: service unit (state/auth rules), handler envelope/status, repository integration (pagination, status filter, tenant isolation, delete counter math, delete during `PROCESSING` leaves no derivatives), and a review assertion that no handler reads bytes.

---

## 3. API surface consumed

```http
GET    /api/v1/events/{eventID}
GET    /api/v1/events/{eventID}/dashboard
GET    /api/v1/events/{eventID}/photos?cursor=&limit=&status=
POST   /api/v1/events/{eventID}/uploads
POST   /api/v1/uploads/{photoID}/url
GET    /api/v1/uploads/{photoID}
POST   /api/v1/uploads/{photoID}/parts
POST   /api/v1/uploads/{photoID}/multipart/complete
POST   /api/v1/uploads/{photoID}/multipart/abort
POST   /api/v1/uploads/{photoID}/complete
GET    /api/v1/photos/{photoID}/url?variant=
DELETE /api/v1/photos/{photoID}
```

No other API or migration changes are expected. If a field is missing, the OpenAPI contract and Go implementation change first.

---

## 4. File map

```text
apps/web/src/app/(dashboard)/events/[eventID]/
  page.tsx                      # tabbed detail (Overview/Photos/Settings/Danger)
apps/web/src/features/photos/
  api.ts                        # upload/library queries + mutations
  keys.ts
  uploader.ts                   # plan, chunk, concurrency, retry, resume, abort
  uploader.test.ts
  storage.ts                    # localStorage registry of in-flight uploads
  errors.ts                     # status/error-code -> user message
  schema.ts                     # client-side validation mirror
  components/
    photos-panel.tsx
    upload-dropzone.tsx
    upload-queue.tsx
    photo-grid.tsx
    photo-tile.tsx
    photo-status-badge.tsx
    delete-photo-dialog.tsx
```

---

## 5. Upload rules

- The browser PUTs to R2 with presigned URLs only. Never send photo bytes to Go or Next; `Idempotency-Key` on initialize/complete.
- Use `XMLHttpRequest` for progress (`fetch` cannot report upload progress).
- Kind selection by declared size: `< 10 MB` simple (one PUT); `>= 10 MB` multipart with the `partSize` from initialize/parts, parts PUT concurrently (3), ETags read from response headers.
- Retry: network/5xx/429 → exponential backoff (1s/2s/4s, max 3 attempts); other 4xx → fail the file, keep the batch running.
- Idempotency: one `crypto.randomUUID()` key per file per phase; retrying initialize/complete with the same key must not create duplicates.
- Resume on mount: read the registry, `GET /uploads/{photoID}` per entry. `UPLOADING` → simple: `POST /uploads/{id}/url`; multipart: `POST /uploads/{id}/parts` for missing parts. `PROCESSING|READY` → drop from queue and refresh the grid. `FAILED` → show failure with retry.
- Cancel: stop XHRs, call multipart abort when applicable, remove the registry entry, then `DELETE /photos/{photoID}` to discard the row and object.
- CORS: R2 bucket must allow `PUT` from the web origin and expose `ETag` to JS. Local MinIO defaults are permissive; the E2E must verify a real browser PUT and ETag read. Document the production bucket CORS rule.
- Signed URLs and upload URLs are never logged or persisted beyond the registry's own short-lived entry (URL + `expiresAt`).

---

## 6. UX rules

- Grid shows the newest photo first; "Load more" or an intersection sentinel; never offset pagination and never full-collection fetches.
- Poll only non-terminal photos (`UPLOADING`/`PROCESSING`) on a 3–5 s interval until they reach `READY`/`FAILED`, then invalidate the dashboard query.
- Tiles render `thumbnail` signed URLs (owner endpoint) once available; use width/height aspect boxes and plain `<img loading="lazy">`, never `next/image` for signed URLs.
- Delete requires confirmation; optimistic removal with rollback on failure; `PHOTO_NOT_FOUND` after a delete is treated as already deleted, not an error toast.
- Empty state explains the direct-to-R2 upload and offers the dropzone.
- Uploading while navigating away aborts XHRs; the registry restores the queue on return (server state is authoritative).
- Client validation is a mirror, not the rule: the API returns `VALIDATION_ERROR` / `UPLOAD_SIZE_MISMATCH` / `IDEMPOTENCY_CONFLICT`; map codes to messages, never render raw server text.
- Tenant isolation: foreign event/photo ids render not-found states, not error toasts.

---

## 7. Test Plan

**Unit (Vitest + RTL)**

- [ ] uploader: simple vs multipart by size; chunk/part math; concurrency cap; retry classification and backoff; resume decisions from status; abort sequence.
- [ ] `storage.ts`: add/update/remove entries; corrupt JSON tolerated; stale entries ignored.
- [ ] `schema.ts`: mime/size/filename rules match the OpenAPI limits.
- [ ] queue renders progress, failed state, retry, cancel; grid renders statuses and empty state; delete dialog blocks until confirmed.
- [ ] error mapping: `EVENT_NOT_FOUND`, `VALIDATION_ERROR`, `UPLOAD_SIZE_MISMATCH`, `IDEMPOTENCY_CONFLICT`, `NOT_SIMPLE`, `PHOTO_NOT_FOUND`.

**E2E (Playwright, live API + Postgres + MinIO + worker)**

- [ ] signup → create event → upload a small JPEG via the dropzone → queue reaches `READY` → tile appears → delete photo → counter updates.
- [ ] reload mid-queue restores the upload and finishes it (assert via registry + grid).
- [ ] multipart path is exercised by Go integration tests (Phase 3) and uploader unit tests; E2E covers simple upload to keep fixtures small.
- [ ] foreign event id renders the not-found state.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # against live Go API + Postgres + MinIO + worker
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
go test ./...
go test -tags=integration ./...
```

---

## 8. Definition of Done

- [ ] All master DoD items that apply, including the §2 backend changes (OpenAPI first, then Go).
- [ ] No hand-written types: every request/response comes from `packages/api-client`.
- [ ] Tenant isolation verified from the frontend (foreign event/photo ids render not-found) and in Go tests.
- [ ] E2E network log proves photo bytes go browser → MinIO/R2 only.
- [ ] No offset pagination; no full-collection fetches; no signed URLs persisted or logged.
- [ ] AGENTS.md updated with any new commands or env vars.
- [ ] Completion report written.
