# Phase 4 — Image Processing Worker & Queue

**Goal:** A PostgreSQL-backed queue and a standalone worker generate thumbnails and optimized images after upload, updating the database so the gallery can serve fast images.

**Depends on:** Phase 3.

**Exit criteria:** Completing an upload leads to derivatives in R2 and `photos.status = READY` within seconds; failures retry with backoff and eventually mark `FAILED`.

---

## 1. Features

- `jobs` table as a durable queue with `FOR UPDATE SKIP LOCKED` polling.
- Worker binary (`cmd/worker`) with graceful shutdown and job concurrency.
- Job types: `image.process`, `object.cleanup` (plus more later phases).
- Image pipeline: download original → validate → read dimensions → generate `thumbnail` (400px), `medium` (1000px), `large/optimized` (2000px) → upload derivatives → update photo row.
- Retry with exponential backoff and max attempts.
- Dead-letter handling: exhausted jobs marked `failed` with reason; photo → `FAILED`.
- Idempotent processing (safe to run twice; overwrite keys).
- Structured job logging (job id, type, photo id, duration).

---

## 2. Database (migration `0005_jobs.sql`)

```sql
CREATE TABLE jobs (
    id UUID PRIMARY KEY,
    type VARCHAR(60) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending|running|done|failed
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    last_error TEXT,
    run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    locked_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_jobs_ready ON jobs(status, run_at) WHERE status = 'pending';
CREATE INDEX idx_jobs_type  ON jobs(type, status);
```

Claim query:

```sql
SELECT * FROM jobs
WHERE status = 'pending' AND run_at <= NOW()
ORDER BY run_at
FOR UPDATE SKIP LOCKED
LIMIT $1;
```

---

## 3. Worker Design

```text
cmd/worker/main.go
  -> queue.Poller (N goroutines)
  -> registry[type] -> Handler
  -> on success: status=done
  -> on error: attempts++, backoff, or status=failed if exhausted
```

Image outputs:

```text
thumbnail  400px  -> thumbnails/{photoID}.webp
medium    1000px  -> medium/{photoID}.webp
large     2000px  -> optimized/{photoID}.webp
original  untouched
```

`imgproc` abstraction so `imaging` (MVP) can be swapped for `libvips`.

---

## 4. Domain Layout

```text
internal/jobs/
  queue.go          # enqueue, claim, complete, fail
  worker.go         # poll loop, concurrency, graceful shutdown
  registry.go       # type -> handler
internal/photos/
  processor.go      # image.process handler
  imagick.go        # decode/resize/encode (interface + impl)
pkg/imaging/        # pure image helpers
```

---

## 5. Reliability Rules

- Exactly one worker owns a claimed job (`SKIP LOCKED` + `locked_by`).
- Processing is idempotent: re-running overwrites derivative keys.
- Download original with streaming/temp file; enforce a max pixel guard to avoid decompression bombs.
- Validate the object is really an image before processing (magic bytes, not just extension).
- On permanent failure, mark photo `FAILED` with a sanitized reason (no internal paths).
- `run_at` backoff: e.g. `min(2^attempts seconds, 5 min)`.

---

## 6. Test Plan

**Unit**
- [ ] queue: enqueue sets pending; claim returns ready jobs only; ordering by `run_at`.
- [ ] retry: error increments attempts and pushes `run_at`; exhaustion marks `failed`.
- [ ] processor: chooses correct output sizes; builds derivative keys; marks photo `READY`.
- [ ] processor idempotency: running twice yields same keys, no duplicate rows.
- [ ] imaging helpers: resize preserves aspect ratio; rejects non-images.

**Integration**
- [ ] claim with two workers never returns the same job (`SKIP LOCKED`).
- [ ] full pipeline against MinIO + Postgres: put original, enqueue, run worker, assert derivatives exist and DB updated with width/height.
- [ ] failure path: corrupt file → photo `FAILED`, job `failed` after max attempts.
- [ ] graceful shutdown leaves no job stuck `running` without release.

**E2E**
- [ ] upload complete → worker run → public metadata reflects `READY` with thumbnail key present.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] Worker runs as a separate Docker service alongside the API.
- [ ] Job observability fields (`attempts`, `last_error`, `locked_by`) verified.
