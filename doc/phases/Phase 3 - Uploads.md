# Phase 3 — Uploads & Object Storage

> **Status: COMPLETE.** See the [Phase 3 Report](Phase 3 - Report.md) for what was built and how it was verified.

**Goal:** The API issues presigned uploads so clients put photo bytes directly into R2. Supports simple and multipart uploads, completion, idempotency, and failure recovery. **The Go API never touches photo bytes.**

**Depends on:** Phase 2.

**Exit criteria:** A client can initialize, upload, complete, and retry an upload; the photo row moves `UPLOADING → READY` (processing enqueued); duplicate `Idempotency-Key` never creates two photos.

---

## 1. Features

- Initialize upload → returns `photoId`, `uploadUrl`, `storageKey`, `expiresAt`.
- Simple presigned PUT for files `< 10 MB`.
- Multipart presigned upload for files `>= 10 MB` (init part URLs, complete multipart, abort).
- Upload completion endpoint: verifies object exists in R2, updates status, enqueues processing.
- Upload status endpoint for polling/recovery.
- `Idempotency-Key` support on initialize + complete.
- File validation: allowed mime types (jpeg, png, webp), max 100 MB, non-empty filename.
- Rejected uploads clean up the orphaned R2 object/photo row (worker).
- Storage accounting increments on completion (detailed reconciliation in Phase 9).

---

## 2. Database (migration `0004_photos.sql`)

```sql
CREATE TABLE photos (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    thumbnail_key TEXT,
    optimized_key TEXT,
    medium_key TEXT,
    original_filename TEXT,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    width INT,
    height INT,
    status VARCHAR(30) NOT NULL,         -- UPLOADING|PROCESSING|READY|FAILED
    upload_kind VARCHAR(20) NOT NULL DEFAULT 'simple',  -- simple|multipart
    multipart_upload_id TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_photos_event_created ON photos(event_id, created_at DESC);
CREATE INDEX idx_photos_event_status  ON photos(event_id, status);

CREATE TABLE upload_idempotency (
    idempotency_key TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE upload_parts (
    photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    part_number INT NOT NULL,
    etag TEXT,
    PRIMARY KEY (photo_id, part_number)
);
```

Storage key convention (enforced by one builder function):

```text
tenant/{userID}/events/{eventID}/originals/{photoID}/{filename}
tenant/{userID}/events/{eventID}/optimized/{photoID}.webp
tenant/{userID}/events/{eventID}/medium/{photoID}.webp
tenant/{userID}/events/{eventID}/thumbnails/{photoID}.webp
```

---

## 3. API Surface

```http
POST /api/v1/events/{eventID}/uploads                  # initialize
POST /api/v1/uploads/{photoID}/parts                    # get presigned part URLs (multipart)
POST /api/v1/uploads/{photoID}/multipart/complete        # finish multipart
POST /api/v1/uploads/{photoID}/multipart/abort           # cancel multipart
POST /api/v1/uploads/{photoID}/complete                  # verify + mark uploaded
GET  /api/v1/uploads/{photoID}                           # status polling
```

Initialize request/response:

```json
{ "filename": "IMG_1234.jpg", "contentType": "image/jpeg", "size": 18273482 }
```

```json
{
  "data": {
    "photoId": "uuid",
    "uploadKind": "multipart",
    "uploadUrl": "https://.../part?uploadId=...",
    "storageKey": "tenant/.../originals/....jpg",
    "expiresAt": "2026-09-12T10:05:00Z",
    "partSize": 10485760
  },
  "error": null
}
```

Client uploads directly to R2, then:

```http
POST /api/v1/uploads/{photoID}/complete
Authorization: Bearer <token>
Idempotency-Key: <key>
```

---

## 4. Domain Layout

```text
internal/uploads/
  service.go        # initialize, parts, complete, abort, validation
  repository.go     # photos, parts, idempotency
  handler.go
internal/photos/
  repository.go     # shared photo read/write
  model.go
pkg/r2/
  client.go         # ObjectStore interface
  s3.go             # aws-sdk-go-v2 S3 client (works with R2 + MinIO)
  keys.go           # storage key builder
```

---

## 5. Reliability Rules

- Validate `size` and `contentType` before issuing any URL.
- Presigned URL TTL short (e.g. 15 min); parts 15 min.
- Complete is idempotent: if photo already `READY`, return current state, do not re-enqueue.
- `Head` the object in R2 on complete; reject if missing or size mismatch.
- On repeated `Idempotency-Key` with a different body → `409 IDEMPOTENCY_CONFLICT`.
- Uploads must survive client restart: status endpoint lets a client resume.
- Never log signed URLs.

---

## 6. Test Plan

**Unit**
- [ ] validation: reject unsupported mime, oversized file, empty/unsafe filename.
- [ ] upload kind selection: `<10MB` simple, `>=10MB` multipart.
- [ ] storage key builder is deterministic and safe (no path traversal).
- [ ] idempotency: same key + same body returns same photo; same key + different body → conflict.
- [ ] complete: object missing → `UPLOAD_NOT_FOUND`; wrong size → `UPLOAD_SIZE_MISMATCH`.
- [ ] complete twice → single processing enqueue.
- [ ] handler envelope/status for all routes.

**Integration**
- [ ] presign + PUT + Head round-trip against MinIO (simple and one multipart part).
- [ ] repository: photos CRUD, part ETag persistence, cascade on event delete.
- [ ] idempotency table unique constraint behavior.
- [ ] tenant isolation: cannot initialize upload for another tenant's event.
- [ ] completion increments `events.storage_bytes` and `photo_count` exactly once.

**E2E**
- [ ] init → PUT to MinIO → complete → photo status becomes `PROCESSING` and a job row exists.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents upload endpoints + idempotency header.
- [ ] No code path reads photo bytes into the API process (verified by review + a test asserting handlers only touch the storage interface).
