# Phase F4 — Public Gallery — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase F4 closes the guest loop: anyone with the `/e/{slug}` link opens a branded, mobile-first gallery with no account, unlocks it when the event is password-protected, lazily loads photos with signed URLs, opens a full-screen viewer, and downloads only when the event settings allow it. The phase also delivered the backend prerequisite that made view-only galleries possible (view variants are no longer gated by `allow_download`) and fixed two cross-cutting issues it surfaced: `X-Gallery-Unlock` was missing from the CORS allowlist, and `packages/api-client` lost error codes on non-2xx envelope responses.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| A guest can open a public event and page through photos lazily | PASS | Server component fetch (`next: { revalidate: 60 }`) + `useInfiniteQuery` `limit=50` cursor; Playwright E2E `public gallery` opens the grid |
| A guest can open the viewer and download per settings | PASS | `PhotoViewer` dialog with keyboard/swipe/focus; E2E clicks Download and receives a `photo-*` download |
| A password event stays blocked until unlocked and rejects wrong passwords | PASS | E2E: gate → wrong password inline error → correct password → tiles visible; Go unit/integration token tests |
| A view-only event (`allowDownload=false`) renders images but offers no download | PASS | Backend §2.1 change + `TestService_PhotoURL_SettingsCombinations` + `TestGalleryService_ViewVsDownloadIntegration`; E2E `view-only gallery` |
| Private/deleted events show not-found | PASS | `visibleEvent` returns 404; `/e/[slug]` calls `notFound()`; E2E `private gallery` sees "Gallery not found" |
| Configured branding appears | PASS | `GalleryHeader` consumes the Phase 7 `branding` DTO as-is; unit tests cover branding fields and null fallback |

---

## 3. What was built

### 3.1 Backend prerequisite — view vs download (§2.1)

- `internal/gallery/service.go`: download gating now applies only to the `original` variant. `thumbnail | medium | large` are view variants and are always served for a visible event; `original` requires `allow_download && allow_original_download`.
- `api/openapi.yaml`: `/url` description, `PhotoVariant`, `PublicPhoto.variants`, and the 403 description updated; `packages/api-client/src/schema.d.ts` regenerated (codegen verified stable).
- Unit `TestService_PhotoURL_SettingsCombinations` (6 settings/variant cases); integration `TestGalleryService_ViewVsDownloadIntegration` (real Postgres + MinIO, fetches each signed URL); Go E2E asserts the view-only case after `PATCH settings`.

### 3.2 Cross-cutting fixes surfaced by the phase

- `pkg/httpx/middleware.go`: `X-Gallery-Unlock` added to `Access-Control-Allow-Headers`; without it the browser preflight blocked every authenticated gallery request (public-only galleries hid the bug).
- `packages/api-client/src/client.ts`: `unwrapEnvelope` now unwraps an envelope-shaped `result.error` (openapi-fetch puts the parsed body there on non-2xx), so `ApiError.code` survives and the gate maps `UNAUTHORIZED` correctly. Test added.

### 3.3 Public route (`apps/web/src/app/e/[slug]/`)

- `page.tsx`: server component fetches `GET /public/events/{slug}` with `next: { revalidate: 60 }`; `404` → `notFound()`.
- `not-found.tsx`: guest-facing not-found state.
- Route stays outside the `proxy.ts` matcher; a signed-in operator sees the guest experience.

### 3.4 Gallery feature (`apps/web/src/features/gallery/`)

- `api.ts` — public client per slug (no token provider), `X-Gallery-Unlock` middleware, `useInfiniteQuery` photos, `useUnlockGallery`, signed-URL queries, per-slug URL cache.
- `unlock.ts` — in-memory mirror + slug-scoped `sessionStorage`, 5 s expiry margin, subscriber notifications, `unlockHeader`.
- `url-cache.ts` — signed URL cache with 30 s expiry margin, single-flight in-flight dedupe, `force` refetch; memory only.
- `errors.ts` / `types.ts` / `keys.ts` — code-mapped messages, normalized `GalleryEvent`/`GalleryPhoto`, query keys.
- `components/gallery-shell.tsx` — orchestrates lock state (`useSyncExternalStore` over the unlock store), photos query, errors, empty state, viewer.
- `components/gallery-header.tsx` — cover (variant fallback), logo/business name, date/location/description, contact links, primary-color CSS variable.
- `components/unlock-gate.tsx` — password form, inline `UNAUTHORIZED` error, missing-gallery state.
- `components/photo-grid.tsx` + `photo-tile.tsx` — aspect-ratio tiles, IntersectionObserver URL fetch, sentinel + accessible "Load more", one retry on image error.
- `components/photo-viewer.tsx` — full-screen dialog (Base UI focus trap/Esc), arrows/swipe, adjacent-photo preload, view-only hides download UI.
- `components/download-button.tsx` — fetches the signed URL, downloads the blob browser → R2 (never via Next/Go).
- `components/gallery-empty.tsx` — no-photos state.

### 3.5 Dashboard copy

- `features/events/share-panel.tsx`: copy now says the gallery is live and that QR codes point at it.

---

## 4. Database changes

None. No migrations.

---

## 5. API surface

No new endpoints. Semantics changed for the existing gallery `url` endpoint:

```http
GET /api/v1/public/events/{slug}/photos/{photoID}/url?variant=thumbnail|medium|large|original
```

- `thumbnail | medium | large`: always served for a visible event (view variants).
- `original`: `403 DOWNLOAD_DISABLED` when `allow_download=false`, `403 ORIGINAL_DOWNLOAD_DISABLED` when `allow_original_download=false`, otherwise 200.
- CORS `Access-Control-Allow-Headers` now includes `X-Gallery-Unlock`.

---

## 6. Files created / modified

```text
api/openapi.yaml
internal/gallery/service.go
internal/gallery/service_test.go
internal/gallery/repository_integration_test.go
test/e2e/gallery_flow_test.go
pkg/httpx/middleware.go
pkg/httpx/httpx_test.go
packages/api-client/src/client.ts
packages/api-client/src/client.test.ts
packages/api-client/src/schema.d.ts
apps/web/src/app/e/[slug]/page.tsx
apps/web/src/app/e/[slug]/not-found.tsx
apps/web/src/features/gallery/api.ts
apps/web/src/features/gallery/errors.ts
apps/web/src/features/gallery/keys.ts
apps/web/src/features/gallery/test-utils.tsx
apps/web/src/features/gallery/types.ts
apps/web/src/features/gallery/unlock.ts
apps/web/src/features/gallery/unlock.test.ts
apps/web/src/features/gallery/url-cache.ts
apps/web/src/features/gallery/url-cache.test.ts
apps/web/src/features/gallery/components/gallery-empty.tsx
apps/web/src/features/gallery/components/gallery-header.tsx
apps/web/src/features/gallery/components/gallery-header.test.tsx
apps/web/src/features/gallery/components/gallery-shell.tsx
apps/web/src/features/gallery/components/photo-grid.tsx
apps/web/src/features/gallery/components/photo-grid.test.tsx
apps/web/src/features/gallery/components/photo-tile.tsx
apps/web/src/features/gallery/components/photo-viewer.tsx
apps/web/src/features/gallery/components/photo-viewer.test.tsx
apps/web/src/features/gallery/components/unlock-gate.tsx
apps/web/src/features/gallery/components/unlock-gate.test.tsx
apps/web/src/features/events/share-panel.tsx
apps/web/vitest.setup.ts
apps/web/e2e/gallery.spec.ts
AGENTS.md
doc/phases/Phase 5 - Report.md
doc/phases/Phase F4 - Public Gallery.md
doc/Implementation Plan.md
```

---

## 7. Tests

### Unit

- Go: gallery `TestService_PhotoURL_SettingsCombinations` covers every `allow_download`/`allow_original_download` combination for view and original variants; existing visibility/token tests updated around the change; `pkg/httpx` preflight test asserts the unlock header is allowed.
- Web (Vitest): `unlock.test.ts` (per-slug storage, expiry, subscriber notification, header only with token, 401 clears); `url-cache.test.ts` (cache hit, expiry margin, single-flight, memory-only); `unlock-gate.test.tsx` (wrong password vs missing event); `photo-grid.test.tsx` (aspect ratio, open index, load more, disabled while fetching, empty state); `photo-viewer.test.tsx` (download visibility matrix, arrows, Esc, focus); `gallery-header.test.tsx` (branding fields, null fallback).
- API client: envelope-in-error-slot test added.

### Integration (`//go:build integration`)

- `TestGalleryService_ViewVsDownloadIntegration`: real Postgres + MinIO; view variants fetched over HTTP with downloads off, original blocked, then allowed as settings change.
- Go E2E `TestE2E_PublicGalleryFlow`: adds the view-only assertions after patching settings.

### E2E

- `apps/web/e2e/gallery.spec.ts` against API + Postgres + MinIO + worker + Next:
  - public: open → tile + image render → viewer → `Download` produces a `photo-*` download;
  - password: blocked → wrong password inline error → correct password → grid;
  - view-only: image renders → viewer has no download controls;
  - private: not-found page.
  - Every test skips gracefully when the API or worker is unreachable.

### Verification commands and results

```text
gofmt -l .                      -> no output
go build ./...                  -> clean
go vet ./...                    -> clean
go vet -tags=integration ./...  -> clean
go test ./...                   -> all packages ok
go test -tags=integration ./... -> all packages ok (Docker; test/e2e re-run with -count=1)
pnpm gen:api                    -> schema.d.ts updated by the F4 descriptions; re-run hash stable
pnpm lint                       -> clean
pnpm typecheck                  -> clean (api-client, ui, web)
pnpm test                       -> 86 tests passed (7 api-client, 79 web)
pnpm build                      -> succeeded; /e/[slug] is server-rendered on demand
pnpm test:e2e (gallery.spec)    -> 4 passed against the live stack (API :18080, worker, MinIO, Next)
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `allowDownload=false` blocked rendering variants, making view-only galleries impossible | Original-only gating in `gallery.PhotoURL`; OpenAPI + tests first |
| Browser preflight rejected `X-Gallery-Unlock` (password galleries only worked server-side) | Added the header to the CORS allowlist + `pkg/httpx` test |
| Non-2xx envelope responses surfaced as `ApiError("UNKNOWN")`, so the gate showed a generic error | `unwrapEnvelope` now unwraps an envelope-shaped `result.error` + test |
| React Compiler `set-state-in-effect` errors in shell/tile | Unlock state read via `useSyncExternalStore`; tiles always use `IntersectionObserver` (stubbed in Vitest) |
| Playwright strict mode matched Next's route-announcer `role="alert"` | Scoped the locator with `hasText` |
| E2E initially hit a stale API binary holding :18080 | Documented the detached-server gotcha in `AGENTS.md` |

---

## 9. Known limitations / follow-ups

- `PublicPhoto` carries no filename, so downloads are named `photo-<id8>`; adding `originalFilename` to the DTO is a small later change if operators ask for it.
- Viewer lookahead preloads adjacent loaded photos only; pressing next at the end triggers load-more rather than auto-advancing into the new page.
- Metadata caching is Next's fetch cache (`revalidate: 60`) only; there is no CDN layer or public bucket by design.
- Public gallery endpoints are still not rate-limited; hardening is Phase 10.
- `watermark_enabled` remains unused by the read path.
- Playwright photo tests skip when the worker/MinIO is not running; they are not part of the default `pnpm test`.

---

## 10. How to try it

```text
# infrastructure
docker compose up -d
goose -dir migrations postgres "postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable" up

# API and worker (separate terminals; 18080 avoids the WinNAT-reserved 8080)
$env:DATABASE_URL="postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable"
$env:R2_ENDPOINT="http://localhost:9000"; $env:R2_ACCESS_KEY="minioadmin"; $env:R2_SECRET_KEY="minioadmin"
$env:R2_BUCKET="cpd-photos"; $env:R2_REGION="us-east-1"; $env:JWT_SECRET="dev-only-change-me"
$env:PUBLIC_BASE_URL="http://localhost:3000"; $env:HTTP_ADDR=":18080"
go run ./cmd/api
go run ./cmd/worker

# web
pnpm dev

# Sign up, create an event, upload a photo, wait for Ready, then open the share link
# (http://localhost:3000/e/{slug}) in a phone-sized viewport. Try public, password,
# view-only (Settings → Allow downloads off), and private visibility.
```
