# Phase F3 — Photo Uploads & Library — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase F3 makes the operator dashboard photo-capable. Operators upload batches straight from the browser to R2/MinIO with presigned URLs, watch per-file progress and retries, see photos move `PROCESSING → READY` without reloading, and delete photos from a cursor-paginated library grid. The phase also delivered the owner-facing photo API it needed: a keyset-paginated photo list, owner/device signed URLs, simple-upload re-presign, and photo deletion with object cleanup and counter adjustments.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Drag a batch of JPEG/PNG/WebP, see per-file progress and retries | PASS | `uploader.ts` engine + 10 unit tests (plan, concurrency, backoff, cancel); queue UI tests |
| Watch `PROCESSING → READY` without reloading | PASS | list polling (`refetchInterval` while non-terminal) + `onChanged` invalidation; live E2E asserted the `Ready` badge |
| Resume an interrupted upload after a page reload | PARTIAL | In-session retries re-use the initialized row via `POST /uploads/{id}/url`; after a full reload the `File` bytes no longer exist, so reconciled rows surface as "interrupted" with Remove (`uploader.reconcile`); selecting the file again starts a new upload. See §9. |
| Delete a photo | PASS | `DELETE /photos/{id}` + optimistic UI + cleanup job; live E2E deleted the uploaded photo |
| Photo bytes flow browser ⇄ R2 only | PASS | E2E asserts no `PUT` request hits the API origin; presigned PUT goes to MinIO |
| Foreign ids surface as not-found | PASS | Go unit/integration tenant-isolation tests; grid renders not-found state |

---

## 3. What was built

### 3.1 Backend prerequisites (API-first)

- `GET /api/v1/events/{eventID}/photos` — owner/device cursor list, newest first, optional `status` filter, `PhotoList` DTO.
- `GET /api/v1/photos/{photoID}/url?variant=` — owner/device signed URL; guests' download settings do not gate the owner.
- `POST /api/v1/uploads/{photoID}/url` — re-presign a simple PUT for `UPLOADING` photos (`NOT_SIMPLE` / `INVALID_UPLOAD_STATE` conflicts).
- `DELETE /api/v1/photos/{photoID}` — aborts multipart, deletes the row, decrements event counters exactly once, and enqueues `object.cleanup` with a key snapshot.
- Repository: `ListByEvent`, `DeleteOwned`, `EventOwnedBy`; cursor helper in `internal/photos/cursor.go` (same opaque format as events/gallery).
- `CleanupPayload` gained `keys[]` so originals and derivatives are removed even after the row is gone.

### 3.2 Uploader engine (`apps/web/src/features/photos/uploader.ts`)

- Framework-free `UploadQueue`: validation, simple vs multipart planning, bounded file concurrency (3), part concurrency (3), retry with exponential backoff, abort/cancel, idempotency keys.
- `transport.ts` uses `XMLHttpRequest` for progress (fetch cannot report upload progress) and reads the `ETag` response header for multipart parts.
- `storage.ts` keeps a `localStorage` registry of in-flight uploads; `reconcile()` resolves rows after a reload.
- Provider keeps the queue above the tabs so uploads survive tab switches; `applyPhotoStatuses` resolves queue rows from the authoritative list.

### 3.3 UI

- Event detail is now tabbed: Overview / Photos / Settings / Danger zone.
- Photos tab: dropzone, upload queue with progress/errors/Retry/Cancel/Remove, cursor-paginated grid with status badges, thumbnail signed URLs per tile, delete confirmation dialog.
- Dashboard aggregates invalidated on upload completion and delete.
- New shared primitives: `packages/ui` `tabs`, `progress`.

### 3.4 Contract tightening

`InitializeUpload`, `UploadStatus`, `UploadPartURLs`, `PartURL`, `SignedURL` now declare `required` fields so generated types stop treating always-present fields as optional.

---

## 4. Database changes

None. No migrations.

---

## 5. API surface added

```http
GET    /api/v1/events/{eventID}/photos?cursor=&limit=&status=
GET    /api/v1/photos/{photoID}/url?variant=thumbnail|medium|large|original
POST   /api/v1/uploads/{photoID}/url
DELETE /api/v1/photos/{photoID}
```

`POST /uploads/{photoID}/url` response:

```json
{ "data": { "uploadUrl": "https://...", "expiresAt": "2026-09-13T10:05:00Z" }, "error": null }
```

---

## 6. Files created / modified

```text
api/openapi.yaml
cmd/api/main.go
internal/photos/cursor.go
internal/photos/handler.go
internal/photos/handler_test.go
internal/photos/queue.go
internal/photos/service.go
internal/photos/service_test.go
internal/photos/cleanup.go
internal/photos/model.go
internal/photos/repository.go
internal/photos/repository_integration_test.go
internal/photos/processor_test.go
internal/photos/processor_integration_test.go
internal/uploads/service.go
internal/uploads/handler.go
internal/uploads/handler_test.go
internal/uploads/fakes_test.go
packages/api-client/src/schema.d.ts
packages/ui/src/components/tabs.tsx
packages/ui/src/components/progress.tsx
apps/web/src/features/photos/api.ts
apps/web/src/features/photos/errors.ts
apps/web/src/features/photos/keys.ts
apps/web/src/features/photos/photo-grid.tsx
apps/web/src/features/photos/photo-status-badge.tsx
apps/web/src/features/photos/photos-panel.tsx
apps/web/src/features/photos/schema.ts
apps/web/src/features/photos/schema.test.ts
apps/web/src/features/photos/storage.ts
apps/web/src/features/photos/transport.ts
apps/web/src/features/photos/upload-dropzone.tsx
apps/web/src/features/photos/upload-provider.tsx
apps/web/src/features/photos/upload-queue.tsx
apps/web/src/features/photos/upload-queue.test.tsx
apps/web/src/features/photos/uploader.ts
apps/web/src/features/photos/uploader.test.ts
apps/web/src/features/events/event-detail.tsx
apps/web/e2e/photos.spec.ts
apps/web/e2e/events.spec.ts
doc/phases/Phase F3 - Photo Uploads.md
doc/phases/Phase F3 - Report.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit

- Go: `internal/photos` service/handler/cleanup/cursor tests; uploads `RePresign` state rules and handler envelope.
- Web (Vitest): `uploader.test.ts` (10) covers validation, simple flow, retry/backoff, non-retryable failure, multipart chunking + sorted parts, cancel, reconcile (interrupted/ready), status sync; `upload-queue.test.tsx` (4) covers progress/actions; `schema.test.ts` (5).

### Integration (`//go:build integration`)

- `ListByEvent`: keyset pagination, status filter, tenant isolation, soft-deleted events.
- `DeleteOwned`: counter math exactly once, `UPLOADING` not counted, keys snapshot, cascade of parts, tenant isolation, repeat delete 404.
- `object.cleanup` queue row payload; cleanup handler deletes provided keys.

### E2E

- `photos.spec.ts` against API + Postgres + MinIO + worker: signup → create event → Photos tab → upload a JPEG → wait for `Ready` → delete → toast + tile removed → no proxied `PUT`.
- `events.spec.ts` updated for the tabbed detail (Settings / Danger zone tabs).

### Verification commands and results

```text
gofmt -l .                      -> no output
go build ./...                  -> clean
go vet ./...                    -> clean
go vet -tags=integration ./...  -> clean
go test ./...                   -> all packages ok
go test -tags=integration ./... -> all packages ok (Docker; first run exposed a pre-existing flake, fixed)
pnpm gen:api                    -> schema.d.ts regenerated, 286-line diff (prettier-formatted)
pnpm lint                       -> clean
pnpm typecheck                  -> clean (api-client, ui, web)
pnpm test                       -> 59 tests passed (6 api-client, 53 web)
pnpm build                      -> succeeded
pnpm test:e2e                   -> 5 passed against the live stack (API :18080, worker, MinIO, Next)
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| No owner-facing photo API existed | Added list/signed-URL/re-presign/delete endpoints and repository methods (OpenAPI first) |
| `object.cleanup` required the photo row to still exist | Payload accepts a key snapshot; deleter ignores missing objects |
| Simple uploads could not resume after a presign expiry | `POST /uploads/{photoID}/url` |
| `localStorage` default parameter crashed SSR | Registry resolves storage lazily |
| Existing events E2E broke when settings moved into tabs | Spec clicks the Settings/Danger zone tabs first |
| Pre-existing flaky `TestProcessor_FailurePathMarksPhotoFailed` (job status read before the poller recorded failure) | `require.Eventually` on the retry transition |
| `@next/next/no-img-element` warning on signed thumbnails | Explicit lint disable with the signed-URL rationale |

---

## 9. Known limitations / follow-ups

- **Cross-reload resume is not possible for in-flight bytes.** Browsers do not persist `File` handles; the registry reconciles `UPLOADING` rows to an "interrupted" item that can be removed, and re-selecting the file starts a new upload. The old row is only removed on user action.
- Multipart uploads are covered by engine unit tests and Go integration tests, not E2E (keeps fixtures small).
- `FAILED` photos (worker exhausted retries) have no reprocess action; the UI surfaces the reason and allows delete. Reprocessing belongs to Phase 9.
- Upload queue rows for successful uploads remain until the list resolves them; they show `Ready` with a Remove action.
- Watermark settings are still not honored by the worker; tracked for Phase 7/later.

---

## 10. How to try it

```text
# infrastructure
docker compose up -d
goose -dir migrations postgres "postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable" up

# API (port 8080 may be reserved; 18080 is the dev default in .env.local)
$env:DATABASE_URL="postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable"
$env:R2_ENDPOINT="http://localhost:9000"; $env:R2_ACCESS_KEY="minioadmin"; $env:R2_SECRET_KEY="minioadmin"
$env:R2_BUCKET="cpd-photos"; $env:R2_REGION="auto"; $env:JWT_SECRET="dev-only-change-me"
$env:PUBLIC_BASE_URL="http://localhost:3000"; $env:HTTP_ADDR=":18080"
go run ./cmd/api

# worker (separate terminal, same env plus DATABASE_URL/R2/JWT_SECRET)
go run ./cmd/worker

# web
pnpm dev
# sign up at http://localhost:3000/signup, create an event, open the Photos tab,
# drop JPEG/PNG/WebP files, watch statuses, then delete one.
```
