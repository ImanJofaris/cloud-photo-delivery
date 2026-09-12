# Phase 4.1 — Lossy WebP & Quality Resampling — Completion Report

**Status:** COMPLETE
**Completed:** 12 September 2026
**Author:** Engineering

---

## 1. Summary

Phase 4.1 fixes the one substantive cost defect left by Phase 4: the image worker encoded **lossless** WebP (`VP8L`) because the chosen library (`HugoSmits86/nativewebp`) exposes no lossy mode. Lossless derivatives are several times larger than a quality-tuned lossy encode, which directly inflates R2 storage and gallery egress — the dominant recurring cost of the product. This phase swaps the encoder to `github.com/gen2brain/vpx` (a pure-Go port of libwebp) and writes **lossy VP8** at per-derivative quality (thumbnail 80, medium 82, optimized 85). It also replaces nearest-neighbour resizing with `draw.ApproxBiLinear` for smoother output. The change stays entirely behind the `imaging.Encoder` / `Decoder` / `Resizer` interfaces, so the processor, job queue and worker are otherwise untouched, and the build remains CGO-free and distroless-compatible.

---

## 2. Exit criteria — verification

This is a follow-up task, not a numbered phase, so acceptance was defined by the work request.

| Criteria | Result | Evidence |
|---|---|---|
| Use `gen2brain/vpx` | PASS | `pkg/imaging/imaging.go` imports `github.com/gen2brain/vpx/webp`; `go.mod` lists it as a direct dependency |
| Per-derivative quality 80/82/85 | PASS | `internal/photos/processor.go` constants `ThumbnailQuality`/`MediumQuality`/`OptimizedQuality`; `TestProcessor_UsesPerDerivativeQuality` asserts `[80 82 85]` |
| Hardcoded quality (not env-configurable) | PASS | Constants in `processor.go`; no config keys added |
| `ApproxBiLinear` resizing | PASS | `Resize` uses `draw.ApproxBiLinear.Scale`; aspect-ratio table still green |
| Change isolated behind imaging interfaces | PASS | `Encoder` changed `Encode(img)` → `Encode(img, quality)`; only `processor.go` call site updated. `Decoder`/`Resizer` signatures unchanged |
| CGO-free Linux build | PASS | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./...` → clean |
| Distroless image build | PASS | `docker build --build-arg TARGET=worker` and `TARGET=api` both succeed; resulting binary runs (config error, not linker error) |
| Derivatives are lossy | PASS | Live smoke: derivative headers are `RIFF/WEBP/VP8 ` (not `VP8L`); `TestStandard_EncodeProducesLossyWebP` asserts the `VP8 ` fourcc |
| Lossy is materially smaller | PASS | `TestStandard_LossyIsSmallerThanLossless`; diagnostic: 2000px optimized 7,998,084 → 1,860,058 bytes (**4.3× smaller** on worst-case noise) |
| All gates green | PASS | gofmt / build / vet / vet-integration / test / test-integration see §7 |

---

## 3. What was built

### 3.1 Encoder swap (`pkg/imaging/imaging.go`)

- Removed `HugoSmits86/nativewebp` and the blank `golang.org/x/image/webp` import.
- `Standard.Encode` now calls `webp.Encode(w, img, webp.EncodeOptions{Quality: quality})`, producing lossy VP8 by default (VP8L only when `Lossless: true`).
- `Encoder` interface changed to `Encode(img image.Image, quality int) ([]byte, error)` so quality is caller-controlled.
- `vpx` registers itself with `image.RegisterFormat`, so WebP decoding continues to work through the existing `Decoder`.

### 3.2 Resampler (`pkg/imaging/imaging.go`)

- `Resize` now uses `golang.org/x/image/draw` `ApproxBiLinear.Scale` instead of hand-rolled nearest-neighbour sampling. Aspect-ratio and no-upscale behaviour are unchanged.

### 3.3 Per-derivative quality (`internal/photos/processor.go`)

- Added `ThumbnailQuality = 80`, `MediumQuality = 82`, `OptimizedQuality = 85`.
- The derivative loop carries a `quality` field per output and passes it to `Encode`.
- Rationale: small derivatives are displayed small, so artifacts are hidden and aggressive compression is safe; the full-screen optimized image gets the highest quality.

### 3.4 Build toolchain fix (`Dockerfile`)

- Bumped base image `golang:1.24-alpine` → `golang:1.26-alpine` to match `go.mod` (`go 1.26.5`). This was pre-existing doc drift (flagged in AGENTS.md) that blocked the distroless build independently of this phase.

---

## 4. Database changes

None.

---

## 5. API surface added

None. The worker-only change is invisible to the HTTP contract.

---

## 6. Files created / modified

```text
pkg/imaging/imaging.go                       (modified: encoder + resizer)
pkg/imaging/imaging_test.go                  (modified: lossy assertions)
pkg/imaging/size_report_test.go              (new: opt-in size diagnostic)

internal/photos/processor.go                 (modified: quality constants + call)
internal/photos/processor_test.go            (modified: quality spy test)

Dockerfile                                   (modified: Go 1.24 -> 1.26)
go.mod / go.sum                              (modified: +vpx, -nativewebp)

doc/phases/Phase 4 - Report.md               (limitations marked resolved)
doc/phases/Phase 4.1 - Report.md             (this file)
doc/Implementation Plan.md                   (stack table, decision log, report list)
```

---

## 7. Tests

### Unit
- `TestStandard_EncodeProducesLossyWebP` — output is `RIFF/WEBP` with a `VP8 ` (lossy) codec fourcc and round-trips through `Decode`.
- `TestStandard_LossyIsSmallerThanLossless` — on high-frequency input, lossy output is smaller than an explicit `Lossless` encode (regression guard for the nativewebp VP8L behaviour).
- `TestStandard_QualityMonotonic` — quality 30 produces a smaller file than quality 95.
- `TestStandard_ResizePreservesAspectRatio` — unchanged table still green under `ApproxBiLinear`.
- `TestProcessor_UsesPerDerivativeQuality` — spy encoder records `[80, 82, 85]`.
- `TestStandard_SizeReport` — opt-in (`IMAGING_SIZE_REPORT=1`) diagnostic that prints real sizes.

### Integration (`//go:build integration`)
- Full processor pipeline against MinIO + Postgres still passes and now writes VP8 derivatives.
- E2E upload→worker→`READY` still passes.

### Verification commands and results

```text
gofmt -l .                                     -> clean
go build ./...                                 -> OK
go vet ./...                                   -> OK
go vet -tags=integration ./...                 -> OK
CGO_ENABLED=0 GOOS=linux go build ./...        -> OK
go test ./...                                  -> all packages PASS
go test -tags=integration -p 1 ./...           -> all packages PASS
docker build --build-arg TARGET=worker .       -> OK (distroless)
docker build --build-arg TARGET=api .          -> OK (distroless)
```

### Measured size impact

Diagnostic on a worst-case 6000×4000 high-frequency image:

```text
thumbnail 400 q80         68706 bytes
medium 1000 q82          377148 bytes
optimized 2000 q85      1860058 bytes
optimized lossless      7998084 bytes   (Phase 4 behaviour)
```

Live smoke (1200×800 JPEG with flat regions — representative of real photos):

```text
thumbnails/...webp   1960 bytes   codec=VP8
medium/...webp       5434 bytes   codec=VP8
optimized/...webp    9106 bytes   codec=VP8
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `nativewebp` writes VP8L (lossless) unconditionally — no quality option | Replaced with `gen2brain/vpx`, which defaults to lossy VP8. |
| Distroless build failed: `go.mod requires go >= 1.26.5 (running go 1.24.13)` | Bumped Dockerfile base to `golang:1.26-alpine` (source of truth is `go.mod`). |
| `vpx` decodes lossy to `*image.NYCbCrA` and lossless to `*image.NRGBA` | Resizer operates on `image.Image` via `draw` scaling, so both are handled. |
| Needed confidence the derivative is actually lossy | Added a `VP8 ` fourcc assertion and a lossy-vs-lossless size test. |

---

## 9. Known limitations / follow-ups

- Quality values (80/82/85) are hardcoded as requested. If operators need per-plan control, move them to `internal/platform/config` (target: Phase 8/10).
- `vpx` is a young project (v0.2.1). It is isolated behind the imaging interfaces and covered by tests, so a revert to `gen2brain/webp` or a `libvips` encoder is a one-file change.
- `EncodeOptions.Method` is left at its default; a slower, more-optimized method could shrink files further at CPU cost.
- No visual-quality/perceptual comparison (e.g. SSIM) is automated; size is asserted, perceptual quality was eyeballed via the smoke image.
- `-race` not run locally (no gcc); CI runs it on Linux.

---

## 10. How to try it

```text
# Size diagnostic (prints per-derivative and lossless sizes):
$env:IMAGING_SIZE_REPORT="1"; go test -v -run TestStandard_SizeReport ./pkg/imaging/

# Full pipeline (same as Phase 4):
copy .env.example .env
make up; make migrate-up; make run          # API
make worker                                  # second terminal
# upload a JPEG and complete (see Phase 3 report), then:
#   GET /api/v1/uploads/{photoId}  -> READY
# download a derivative from MinIO (:9001) and confirm the header is RIFF/WEBP/VP8

# Distroless build check:
docker build --build-arg TARGET=worker -t cpd-worker .
docker build --build-arg TARGET=api -t cpd-api .
```
