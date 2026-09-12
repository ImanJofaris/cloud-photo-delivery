# Phase 4 — Image Processing Worker & Queue — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Author:** Engineering

---

## 1. Summary

Phase 4 delivered the asynchronous image pipeline behind uploads. A PostgreSQL-backed `jobs` queue is claimed with `FOR UPDATE SKIP LOCKED`, and a standalone worker binary (`cmd/worker`) polls it, downloads each original from object storage, validates and decodes it, generates three WebP derivatives (thumbnail 400px, medium 1000px, optimized 2000px), uploads them, and transitions the photo to `READY` with its dimensions and derivative keys. Failures retry with exponential backoff and, when attempts are exhausted, the job is marked `failed` and the photo `FAILED`. The gallery can now serve fast, sized images instead of originals, and upload completion is fully decoupled from processing.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Completing an upload leads to derivatives in R2 and `photos.status = READY` within seconds | PASS | `TestE2E_UploadProcessingFlow`: real JPEG → MinIO → complete (`PROCESSING`) → poller runs → `READY`; `TestProcessor_PipelineAgainstMinioAndPostgres` asserts all three keys exist via `Head` |
| Failures retry with backoff | PASS | `TestQueue_FailRetriesWithBackoffThenExhausts` (attempts 1→`run_at = now + 1s`, 2→`now + 2s`); `TestProcessor_FailurePathMarksPhotoFailed` observes `attempts = 1`, status `pending` on first failure |
| Failures eventually mark `FAILED` | PASS | `TestQueue_FailRetriesWithBackoffThenExhausts` (3rd attempt → jobs.status `failed`, `last_error` recorded); processor marks photo `FAILED` on corruption |
| `jobs` table durable queue with `FOR UPDATE SKIP LOCKED` | PASS | `TestQueue_ConcurrentClaimNeverReturnsSameJob`: 8 workers, 20 jobs, each claimed exactly once |
| Worker binary with graceful shutdown and job concurrency | PASS | `cmd/worker/main.go`; `TestPoller_ShutdownReleasesInFlightJob` (released, not failed); `TestPoller_ShutdownReleasesInFlightJob`; concurrency via `WORKER_CONCURRENCY` |
| Job types `image.process`, `object.cleanup` | PASS | Worker registers both `PROCESS_PHOTO`/`image.process` → processor and `object.cleanup` → `CleanupHandler`; `registry.Types()` asserted |
| Image pipeline: validate → dimensions → thumbnail/medium/large → upload → update row | PASS | `TestProcessor_GeneratesDerivativesAndMarksReady`; `TestProcessor_RejectsCorruptImage` |
| Idempotent processing (safe to run twice) | PASS | `TestProcessor_IdempotentWhenAlreadyReady` (no re-download); `TestProcessor_PipelineAgainstMinioAndPostgres` re-runs cleanly |
| Dead-letter: photo → `FAILED`, sanitized reason | PASS | `CleanupHandler` + processor `MarkFailed`; worker logs sanitized error text |
| Structured job logging (id, type, photo, duration) | PASS | `internal/jobs/worker.go` logs `job_id`, `job_type`, `attempt`, `duration_ms`; processor logs `photo_id`, dimensions |
| Worker runs as a separate Docker service | PASS | `docker compose --profile app up` builds `api` + `worker` from the shared `Dockerfile` (`TARGET` arg); `docker compose config` validates |
| Job observability fields verified | PASS | `attempts`, `last_error`, `locked_at`, `locked_by` asserted in `internal/jobs/queue_integration_test.go` |
| All master DoD gates | PASS | gofmt / build / vet / vet-integration / test / test-integration all clean |

---

## 3. What was built

### 3.1 Jobs package (`internal/jobs/`)

| File | Responsibility |
|---|---|
| `job.go` | `Job` model, `Status` constants, `Queue` interface (`Claim`/`Complete`/`Fail`/`Release`). |
| `queue.go` | `PostgresQueue`: `Enqueue`, atomic `Claim` (single tx: `SELECT ... FOR UPDATE SKIP LOCKED` then `UPDATE ... RUNNING`), `Complete`, `Fail` (attempt++, backoff or terminal), `Release`, and capped exponential `Backoff`. |
| `registry.go` | `Registry` mapping job type → handler, `Dispatch`, `ErrUnknownJobType`, sorted `Types()` for logging. |
| `worker.go` | `Poller`: N concurrent poll loops, graceful shutdown, runs handlers on a child context, completes on success, fails/retries on error, releases in-flight jobs on shutdown (no attempt counted). |

### 3.2 Imaging package (`pkg/imaging/`)

- `imaging.go` — pure decode/resize/encode helpers: `Standard` implements `Decoder`, `Encoder`, `Resizer`. Decodes JPEG/PNG/WebP via registered codecs, enforces a `MaxPixels` (80 MP) decompression-bomb guard, rejects non-images (`ErrUnsupported`), resizes preserving aspect ratio without upscaling, and encodes WebP with `github.com/HugoSmits86/nativewebp` (pure Go). Interfaces allow swapping in a `libvips`-backed implementation.

### 3.3 Photos processor (`internal/photos/`)

| File | Responsibility |
|---|---|
| `processor.go` | `Processor.Handle` — the job handler. Loads the photo, short-circuits `READY`/missing, resolves the event owner, downloads the original, decodes/validates, generates and uploads three derivatives, marks `READY` with width/height/keys. Depends only on interfaces (`Repository`, `PhotoStore`, imaging interfaces, `EventOwner`). |
| `derivative.go` | `Derivative`/`Derivatives`, `ProcessPayload`, `ErrNotProcessing`. |
| `cleanup.go` | `CleanupHandler` (`object.cleanup`) deletes an orphaned original, idempotent on missing objects/photos. |
| `repository.go` (modified) | Added `MarkReady` (records dimensions + derivative keys, clears error) and `EventOwner` (scoped user lookup for key rebuilding). |

### 3.4 Wiring

- `cmd/worker/main.go` builds config, DB pool, R2 store, photo repo, `imaging.Standard`, the `Processor`, the registry (both job-type spellings + cleanup), and the poller; runs until SIGINT/SIGTERM then drains.
- `internal/uploads/queue.go` now delegates to `jobs.PostgresQueue`, enqueuing the typed `photos.ProcessPayload` under `PROCESS_PHOTO`.
- `internal/platform/config` adds `WORKER_CONCURRENCY` (default 2) and `WORKER_POLL_INTERVAL` (default 1s).
- `docker-compose.yml` adds optional `api` and `worker` services under the `app` profile, both built from the existing `Dockerfile`.

---

## 4. Database changes

```text
migrations/0005_jobs.sql
```

- `ALTER TABLE jobs ADD COLUMN max_attempts INT NOT NULL DEFAULT 5`.
- `ALTER TABLE jobs ADD COLUMN locked_at TIMESTAMPTZ`.
- `ALTER TABLE jobs ADD COLUMN locked_by TEXT`.
- Drops `idx_jobs_pending`; adds `idx_jobs_ready` (partial, `WHERE status = 'pending'`) and `idx_jobs_type`.

The `jobs` table itself was created in Phase 3 (`0004_photos.sql`) with `id/type/payload/status/run_at/attempts/last_error`; Phase 4 adds the locking and retry-cap fields. Rollback (up→down) drops the three columns and restores `idx_jobs_pending`, covered by `TestMigrations_JobsUpDownRoundTrip`.

---

## 5. API surface added

None. Phase 4 is worker-only; the HTTP contract is unchanged. The worker consumes the `PROCESS_PHOTO` job already enqueued by `POST /api/v1/uploads/{photoID}/complete` (Phase 3), so `api/openapi.yaml` is not modified.

Derivative storage keys (already defined in `pkg/r2/keys.go`):

```text
tenant/{userID}/events/{eventID}/thumbnails/{photoID}.webp   # 400px
tenant/{userID}/events/{eventID}/medium/{photoID}.webp       # 1000px
tenant/{userID}/events/{eventID}/optimized/{photoID}.webp    # 2000px
```

---

## 6. Files created / modified

```text
migrations/0005_jobs.sql
migrations/migrations_integration_test.go              (modified)

internal/jobs/job.go
internal/jobs/queue.go
internal/jobs/registry.go
internal/jobs/worker.go
internal/jobs/registry_test.go
internal/jobs/worker_test.go
internal/jobs/queue_integration_test.go

internal/photos/processor.go
internal/photos/cleanup.go
internal/photos/derivative.go
internal/photos/processor_test.go
internal/photos/processor_integration_test.go
internal/photos/repository.go                          (modified)
internal/photos/repository_integration_test.go         (modified)

internal/uploads/queue.go                              (modified)
internal/uploads/fakes_test.go                         (modified)

internal/platform/config/config.go                     (modified)
cmd/worker/main.go                                     (modified)
docker-compose.yml                                     (modified)

pkg/imaging/imaging.go
pkg/imaging/imaging_test.go

test/e2e/processing_flow_test.go
test/e2e/uploads_flow_test.go                          (modified)
go.mod / go.sum                                        (nativewebp, x/image)
```

---

## 7. Tests

### Unit
- jobs: `Backoff` table (exponential, capped at 5 min, overflow-safe); registry register/lookup/dispatch/unknown/error propagation; poller success→done, failure recording, unknown type, graceful-shutdown release, cancel-before-claim.
- processor: derivative generation and keys, `READY` transition with dimensions, idempotent on `READY`, missing-photo no-op, corrupt image rejection, download failure, `FAILED` guard, invalid payload; cleanup happy path, missing photo, invalid payload, repo error.
- imaging: JPEG/PNG decode, non-image rejection, aspect-ratio resize table (landscape/portrait/square/no-upscale), WebP magic bytes + round-trip.

### Integration (`//go:build integration`)
- jobs queue: claim sets running + lock; empty → `ErrNoJob`; ordering by `run_at`; future jobs not claimed; 8-way concurrent claim returns each job once; complete clears lock; fail retry/backoff/exhaustion; release without attempt.
- photos processor: full pipeline against real MinIO + Postgres asserting `READY`, dimensions, three keys, and re-run idempotency; corrupt-file failure path marking photo `FAILED`; 4 concurrent workers process 5 photos to `READY` with exactly 5 `done` jobs.
- migrations: `0005` up/down round-trip on the added columns.
- E2E: signup → event → init → real JPEG PUT to MinIO → complete → run worker → `READY` with thumbnail key and status endpoint.

### Verification commands and results

```text
gofmt -l .                            -> clean
go build ./...                        -> OK
go vet ./...                          -> OK
go vet -tags=integration ./...        -> OK
go test ./...                         -> all packages PASS
go test -tags=integration -p 1 ./...  -> all packages PASS
go build ./cmd/worker                 -> OK
docker compose config                 -> valid
```

Service coverage: `processor.go` 87.5%, `cleanup.go` 93.3%, `pkg/imaging` 82.9% (floor 80%).

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `PROCESS_PHOTO` (enqueued in Phase 3) vs `image.process` (phase doc) naming mismatch | Worker registers both type strings to the same handler; Phase 3's enqueue path is unchanged. |
| Phase 3's `jobs` table lacked locking/retry-cap columns | Added them in `0005_jobs.sql` rather than editing the applied `0004`. |
| `nativewebp` only offers lossless WebP and has no `Quality` option | `Encoder.Encode(img)` signature dropped quality; the library's default compression is used. |
| `photos.Repository` is implemented by the uploads test fake | Added `MarkReady` and `EventOwner` to `internal/uploads/fakes_test.go`. |
| PowerShell mangled `-coverprofile`/`-func` args (split on `.`) | Used the call operator with an argument array for coverage runs. |
| `image.Image` cannot satisfy a custom `Bounds()` interface in the first processor draft | Rewrote `generate` to accept `image.Image` directly. |

---

## 9. Known limitations / follow-ups

- ~~WebP encoding is lossless (nativewebp), so derivatives may be larger than a quality-tuned lossy encode.~~ **Resolved in Phase 4.1** — see [Phase 4.1 Report](Phase 4.1 - Report.md): switched to lossy VP8 (gen2brain/vpx) with per-derivative quality 80/82/85.
- ~~Resize uses nearest-neighbour sampling~~ **Resolved in Phase 4.1** — now `draw.ApproxBiLinear`.
- Retry schedule is `min(2^(attempts-1)s, 5m)` with a fixed `max_attempts = 5`; per-type tuning is not yet configurable.
- The `object.cleanup` handler is registered but nothing enqueues it yet; orphan sweeps for abandoned uploads are Phase 9.
- Processing failure does not delete derivatives partially written before the failure; re-runs overwrite keys, so this is harmless but leaves stray objects if the photo is later purged (Phase 9).
- `-race` not run locally (no gcc); CI runs it on Linux.

---

## 10. How to try it

```text
copy .env.example .env
make up
make migrate-up
make run                      # API (set HTTP_ADDR=:18080 if 8080 is reserved)
make worker                   # in a second terminal

# sign up, create an event, initialize + complete an upload (see Phase 3 report),
# then PUT a real JPEG to the returned uploadUrl before completing.

# watch the worker log a PROCESS_PHOTO job and the photo become READY:
curl http://localhost:8080/api/v1/uploads/<photoId> \
  -H "Authorization: Bearer <accessToken>"

# confirm derivatives exist (MinIO console at http://localhost:9001):
#   tenant/<user>/events/<event>/thumbnails/<photo>.webp
#   tenant/<user>/events/<event>/medium/<photo>.webp
#   tenant/<user>/events/<event>/optimized/<photo>.webp

# or run the whole stack in Docker:
docker compose --profile app up --build
```
