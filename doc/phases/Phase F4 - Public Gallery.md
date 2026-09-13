# Phase F4 — Public Gallery

**Goal:** Guests open `/e/{slug}` on a phone with no account, unlock password-protected galleries, browse a branded, lazy-loading photo grid, view photos full screen, and download when the event permits.

**Depends on:** Phase F0 (env, api-client); Phase 5 (public gallery API); Phase 7 (branding in the public DTO); backend prerequisite in §2.1.

**Exit criteria:** A guest can open a public event, page through photos lazily, open the viewer, and download per settings; a password event stays blocked until unlocked and rejects wrong passwords; a view-only event (`allowDownload=false`) still renders images but offers no download; private/deleted events show not-found; configured branding appears.

---

## 1. Features

- Public route `app/e/[slug]/` — no auth, no dashboard chrome, mobile-first, reachable from the F2 share link and Phase 7 QR codes.
- Server-rendered event metadata (name, date, location, description, cover photo, branding) with the API's 60 s cache semantics.
- Password flow: `requiresUnlock` → unlock gate → event-scoped token → `X-Gallery-Unlock` on subsequent calls → silent re-gate on 401.
- Cursor-paginated photo grid with infinite scroll (plus an accessible "Load more" fallback), aspect-ratio placeholders from width/height.
- Signed URL per visible photo via `IntersectionObserver`; in-memory cache with an expiry margin; silent refetch when a URL expires.
- Full-screen viewer: keyboard (`←`/`→`/`Esc`), swipe, focus trap, download action.
- Download gating driven by `allowDownload` / `allowOriginalDownload`; view-only events render with no download affordance.
- Branded header: business name, logo, primary/secondary colors, contact links.
- Empty (no photos yet), not-found, network-error, and locked states.

---

## 2. Backend prerequisites

### 2.1 Gallery view vs download (must land first)

`gallery.PhotoURL` currently rejects every variant when `allowDownload=false` (`internal/gallery/service.go:176`; flagged as a follow-up in `doc/phases/Phase 5 - Report.md`), which would make view-only galleries unrenderable.

- Rule: `thumbnail | medium | large` are **view** variants and are always served for a visible event; `original` requires `allowDownload && allowOriginalDownload`.
- `allowDownload=false` hides download affordances; it must not hide images. `DOWNLOAD_DISABLED` no longer applies to view variants.
- Update the OpenAPI `/url` description and the `PublicPhoto.variants` description ("available for viewing", not "for download"); regenerate `packages/api-client`.
- Unit + integration tests for every valid settings combination (view variants always served; `original` only when both flags are on); the F4 E2E asserts the view-only case.
- Update the Phase 5 report follow-up to point at this phase.

### 2.2 Branding contract (produced by Phase 7)

F4 renders whatever Phase 7 puts on `PublicEvent`; Phase 7 must provide, on the gallery DTO:

```json
{
  "branding": {
    "businessName": "string | null",
    "logoUrl": "string | null",
    "primaryColor": "#rrggbb | null",
    "secondaryColor": "#rrggbb | null",
    "contactEmail": "string | null",
    "contactPhone": "string | null",
    "websiteUrl": "string | null"
  }
}
```

- `branding` itself is optional/nullable; F4 falls back to event metadata and default theme tokens.
- `logoUrl` must be safe inside a response the browser may cache for 60 s. If Phase 7 returns a short-lived signed URL, F4 must fetch it separately per render rather than embed it in cached metadata. Decide this in Phase 7 and keep F4 consuming only the DTO.
- QR codes point at `/e/{slug}` with no required params; F4 ignores unknown query params (`?src=qr` is reserved for Phase 9 analytics).

No other API changes are needed. If a field is missing, the OpenAPI contract and Go implementation change first.

---

## 3. API surface consumed

```http
GET  /api/v1/public/events/{slug}
POST /api/v1/public/events/{slug}/unlock
GET  /api/v1/public/events/{slug}/photos?cursor=&limit=
GET  /api/v1/public/events/{slug}/photos/{photoID}
GET  /api/v1/public/events/{slug}/photos/{photoID}/url?variant=thumbnail|medium|large|original
```

All calls go browser → Go directly with no auth header. `X-Gallery-Unlock` is sent only for password events.

---

## 4. File map

```text
apps/web/src/app/e/[slug]/
  page.tsx                      # server component: metadata + gallery shell
  not-found.tsx
apps/web/src/features/gallery/
  api.ts                        # public queries/mutations, no token provider
  keys.ts
  unlock.ts                     # per-slug token storage + header provider
  unlock.test.ts
  url-cache.ts                  # signed URL cache, expiry margin, single-flight
  url-cache.test.ts
  errors.ts
  components/
    gallery-shell.tsx           # client orchestrator
    gallery-header.tsx          # branding + event info + cover
    unlock-gate.tsx
    photo-grid.tsx
    photo-tile.tsx              # intersection-driven URL fetch
    photo-viewer.tsx
    download-button.tsx
    gallery-empty.tsx
```

---

## 5. Rendering and data rules

- Create a public client with `createApiClient({ baseUrl: apiVersionedBaseUrl() })` and **no** token provider; never route public calls through the authenticated `apiCall` helper.
- First paint: the server component fetches `GET /public/events/{slug}` with `next: { revalidate: 60 }`. `EVENT_NOT_FOUND` → `notFound()`. Password events return metadata with `requiresUnlock: true`.
- Photos: client `useInfiniteQuery`, `limit=50`, cursor only. Render the grid only after unlock when `requiresUnlock`.
- Unlock token: keep in memory and mirror to `sessionStorage` under a slug-scoped key (never `localStorage`); on 401 clear it and show the gate again.
- Signed URLs: fetch only when a tile approaches the viewport, one request per tile per variant. Cache in memory with `expiresIn - 30 s`; never persist. On 403/expired, refetch once and retry the image.
- Images: plain `<img loading="lazy" decoding="async">` with explicit width/height aspect boxes; no `next/image` for signed or short-lived URLs.
- The viewer preloads `large` URLs for the adjacent photos only (bounded lookahead).
- Respect API caching: metadata is cacheable (60 s); `/url` and password responses are never cached. Never add `Cache-Control` overrides for credential-bearing responses.

---

## 6. UX rules

- Mobile-first: 2–3 column grid, tap targets >= 44 px, no horizontal scroll, safe-area padding; desktop gets a wider grid.
- Viewer: swipe on touch, arrows on keyboard, `Esc`/back button closes, focus trapped while open, reduced-motion respected.
- Downloads: `allowDownload` enables derivative downloads (`large`); `original` additionally requires `allowOriginalDownload`. When disabled, hide the control entirely — never show a disabled button that explains a server rule.
- Branding: primary color applied as an accent (CSS variable on the gallery wrapper); logo in the header with the business name as text fallback; contact links only when present.
- Errors: 404 renders the not-found page; wrong password is an inline form error mapped from `UNAUTHORIZED`; network failures get a retry control. Never render raw server messages.
- Accessibility: `alt` text built from the event name and photo position, visible focus rings, landmarks for header/grid/viewer.
- Auth is irrelevant here: a signed-in operator opening `/e/{slug}` sees exactly the guest experience. Keep the route outside the `proxy.ts` matcher.

---

## 7. Test Plan

**Unit (Vitest + RTL)**

- [ ] `unlock.ts`: token stored per slug, expiry handling, cleared on 401, header only for password events.
- [ ] `url-cache.ts`: cache hit/miss, expiry margin, in-flight dedupe, no persistence.
- [ ] unlock gate: wrong password (`UNAUTHORIZED`) vs missing event (`EVENT_NOT_FOUND`) rendering.
- [ ] grid: tiles render aspect ratios, load-more/sentinel, empty state, view-only has no download UI.
- [ ] viewer: next/prev/escape, download visibility by `allowDownload`/`allowOriginalDownload`, focus behavior.
- [ ] header: branding fields render; defaults when `branding` is null.

**E2E (Playwright, live API; photos need MinIO + worker)**

- [ ] Public event seeded with one photo: open `/e/{slug}` → grid renders the image → open viewer → download `large`.
- [ ] Password event: blocked → wrong password inline error → correct password → photos visible.
- [ ] View-only event (`allowDownload=false`): image renders, no download control.
- [ ] Private event: not-found page.
- [ ] Skips gracefully when the API (and MinIO/worker for seeding) is unreachable.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # against live Go API + Postgres (MinIO + worker for photo seeding)
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
go test ./...
go test -tags=integration ./...
```

---

## 8. Definition of Done

- [ ] All master DoD items that apply, including the §2.1 backend fix (OpenAPI first, then Go).
- [ ] Phase 7 branding contract consumed as-is; no hand-written duplicate types.
- [ ] No `next/image` for signed URLs; no signed URLs persisted or logged; no bytes through Next.
- [ ] `pnpm test:e2e` covers the phase's critical guest flow (view, unlock, view-only, not-found).
- [ ] F2 share panel copy updated — the gallery is live, QR codes link to it.
- [ ] AGENTS.md updated with any new commands or env vars.
- [ ] Completion report written.
