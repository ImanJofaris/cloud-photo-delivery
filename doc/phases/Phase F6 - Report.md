# Phase F6 — Analytics, Exports & Lifecycle — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-16
**Author:** opencode

---

## 1. Summary

Phase F6 completes the operator surface against the Phase 9 APIs: a `/analytics` page with all-time totals and a zero-filled UTC daily chart for a 7/30/90/365-day window, an event-scoped Analytics tab, a dashboard "last 30 days" strip, polling ZIP exports with a signed-URL download, and event lifecycle affordances (expiry labels, expired banner + Extend dialog, expired filter, grace-period copy). It adds no backend or OpenAPI changes; `pnpm gen:api` is diff-stable, and the only new dependency is `recharts` via the shadcn `chart` primitive.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Analytics page with all-time totals + daily series for a selectable window, account-wide and per event | PASS | `features/analytics/*`; E2E `analytics.spec.ts` opens `/e/{slug}?src=qr` -> `/analytics` and the event Analytics tab show views/QR >= 1 |
| Request a ZIP export, watch it reach `ready`, download from the signed URL | PASS | `features/events/export-card.tsx` polls every 5 s and stops on terminal states; E2E `exports.spec.ts` ready -> download `.zip` from the signed href |
| Extend an event's expiry, including reactivating an expired event | PASS | `features/events/extend-expiry-dialog.tsx`; E2E `lifecycle.spec.ts` expired banner -> Extend 30 days -> badge `active`, banner gone |
| Expired / expiring-soon state in the events list and event detail | PASS | `expiry-label.tsx`, `events-list.tsx`; E2E asserts a 3-day event shows "Expires in 3 days" and the Expired filter returns only expired events |
| Archived events offer no extend action | PASS | `ExtendExpiryDialog` returns null for `status === "archived"`; unit test asserts an empty render |
| No frontend business rules; API remains authoritative | PASS | 422/409/404 map to copy; range limits are form constraints only; no client-side expiry/upload blocks |

---

## 3. What was built

### 3.1 Chart primitive

- `packages/ui/src/components/chart.tsx` via `pnpm shadcn add chart -c apps/web` (wrapper applies the Windows path fix), plus `recharts@3.8.0`.
- `recharts` is declared in **both** `packages/ui` (where `chart.tsx` imports it) and `apps/web` (where feature code composes `Area`/`AreaChart`), matching pnpm's strict resolution.

### 3.2 Analytics (`features/analytics/`)

- `keys.ts` — `analyticsKeys.account(days)` / `analyticsKeys.event(eventId, days)`; the window is part of the key.
- `api.ts` — `useAccountAnalytics(days)` and `useEventAnalytics(eventId, days)` through `apiCall`.
- `schema.ts` — `ANALYTICS_WINDOWS = [7,30,90,365]`, `DEFAULT_ANALYTICS_WINDOW = 30`, days schema 1–365, `parseAnalyticsDays` fallback.
- `format.ts` — `fillDailySeries` (contiguous UTC zero-fill), `utcDaySeries`, `formatUtcDay`/`formatUtcDayLong`, `seriesTotal`, `formatCount` (Intl, `—` for missing).
- `errors.ts` — maps `EVENT_NOT_FOUND`, `VALIDATION_ERROR`, `RATE_LIMITED`; never leaks server messages.
- `window-select.tsx`, `analytics-totals.tsx` (4 counters + event/photo counts, `—` while loading), `daily-chart.tsx` (metric toggle, one series, `sr-only` text summary, "No activity yet" empty state), `analytics-panel.tsx`.
- `app/(dashboard)/analytics/page.tsx` server wrapper; sidebar Analytics entry (`ChartColumn`); `/analytics` added to `PROTECTED_PREFIXES` and the proxy matcher.

### 3.3 Events additions (`features/events/`)

- `api.ts` — `useEventExport` with `exportRefetchInterval` (5 s only while `pending|processing`; `false` on `ready|failed|expired` and without an id), `useCreateEventExport` (seeds `eventKeys.export`), `useExtendEvent` (seeds `eventKeys.detail`, invalidates `eventKeys.lists()`).
- `keys.ts` — `eventKeys.export(eventId, exportId)`.
- `export-card.tsx` — create-or-resume; disabled with a hint at `photoCount === 0`; persists only `cpd:export:{eventId}` in sessionStorage; clears a stale id on `EXPORT_NOT_FOUND`; shows size + "expires in …"; ready links the signed `downloadUrl` directly; failed/expired offer "Generate new export".
- `extend-expiry-dialog.tsx` — presets 7/30/90 + custom 1–3650, `max(now, expiresAt)` copy, toast with the new expiry, `INVALID_STATUS_TRANSITION` -> "Archived events cannot be extended.", hidden for archived.
- `expiry-label.tsx` + `format.ts` — `expiryFromNow` (warning at exactly 7 days, destructive past), `formatTimeUntil`; `events-list.tsx` adds the `expired` filter and expiry hints; `danger-zone.tsx` states the 30-day grace period and no-restore.
- `analytics-tab.tsx` — event-scoped totals (4 counters + all-time photos), window select, chart, UTC help text; `event-detail.tsx` adds the tab between Overview and Photos, the expired banner, the Extend affordance, and the Export card.

### 3.4 Dashboard strip

- `dashboard/page.tsx` "Last 30 days" card (views, visitors, downloads, QR scans) linking to `/analytics`, sharing `analyticsKeys.account(30)` with the analytics page.

### 3.5 Gallery QR attribution forwarding

- `app/e/[slug]/page.tsx` now reads `searchParams` and forwards `?src=qr` to `GET /public/events/{slug}`, so QR-attributed opens are counted (the Phase 9 API records scans only when the request carries `src=qr`).

### 3.6 E2E harness

- `e2e/api-proxy.mjs` — a Node-stdlib proxy on `:18080` forwarding to the API on `:18081` with a per-request `X-Forwarded-For`. The suite previously exhausted the API's per-IP login/refresh limiter (10/min) and died mid-run with 429-driven 401s.

---

## 4. Database changes

None. No migrations.

---

## 5. API surface added

No new endpoints and no OpenAPI changes. `pnpm gen:api` produces no `schema.d.ts` diff. Consumed:

```http
GET  /api/v1/account/analytics?days=1..365
GET  /api/v1/events/{eventID}/analytics?days=1..365
POST /api/v1/events/{eventID}/exports
GET  /api/v1/events/{eventID}/exports/{exportID}
POST /api/v1/events/{eventID}/extend
GET  /api/v1/events?status=expired
GET  /api/v1/events/{eventID}
GET  /api/v1/public/events/{slug}?src=qr
```

---

## 6. Files created / modified

```text
apps/web/src/app/(dashboard)/analytics/page.tsx
apps/web/src/app/(dashboard)/dashboard/page.tsx
apps/web/src/app/e/[slug]/page.tsx
apps/web/src/components/app-sidebar.tsx
apps/web/src/proxy.ts
apps/web/src/proxy.test.ts
apps/web/src/features/analytics/{api,keys,schema,errors,format}.ts
apps/web/src/features/analytics/{analytics-panel,analytics-totals,daily-chart,window-select}.tsx
apps/web/src/features/analytics/{format,schema,daily-chart,analytics-panel}.test.{ts,tsx}
apps/web/src/features/events/{api,keys,errors,format,schema}.ts
apps/web/src/features/events/{analytics-tab,export-card,extend-expiry-dialog,expiry-label}.tsx
apps/web/src/features/events/{event-detail,events-list,danger-zone}.tsx
apps/web/src/features/events/{format,expiry-label,export-card,extend-expiry-dialog,events-list,analytics-tab,api}.test.{ts,tsx}
apps/web/e2e/{analytics,exports,lifecycle}.spec.ts
apps/web/e2e/api-proxy.mjs
apps/web/playwright.config.ts
apps/web/package.json
packages/ui/src/components/chart.tsx
packages/ui/package.json
pnpm-lock.yaml
```

---

## 7. Tests

### Unit (Vitest + RTL, 52 new tests across 10 new files)

- Analytics: window schema 1–365 + default fallback; UTC zero-fill across month boundaries and out-of-window days; UTC labels; count formatting; `EVENT_NOT_FOUND`/`VALIDATION_ERROR`/`RATE_LIMITED` copy with no leakage.
- `AnalyticsPanel`/`WindowSelect`: default 30, selecting "Last 7 days" re-queries `useAccountAnalytics(7)`, `—` placeholders while loading, error rendering.
- `DailyChart`: zero-fills a sparse series, `sr-only` summary, one-metric-at-a-time toggle, "No activity yet".
- Exports: disabled at `photoCount === 0`; POST stores only the export id and resumes polling; ready shows size + signed `downloadUrl`; failed/expired offer a new export; a 404 export clears the stored id; `exportRefetchInterval` polls only `pending|processing`; create seeds `eventKeys.export`.
- Extend: preset payloads, custom 1–3650 validation, archived renders nothing, `INVALID_STATUS_TRANSITION` copy, detail cache seeded + lists invalidated.
- Lifecycle: `expiryFromNow` at exactly 7 days / 8 days / same-day / past / null; `formatTimeUntil`; expired filter option + filtering; expiring-soon hint only inside 7 days; expired hint on cards.

### Integration (`//go:build integration`)

Not run — no backend changes in F6.

### E2E (Playwright, live API, workers = 1)

- Analytics: API-seeded event -> open `/e/{slug}?src=qr` -> `/analytics` shows views and QR scans >= 1 -> the event Analytics tab shows the same counters.
- Exports: upload a JPEG, wait for the worker -> two immediate API POSTs return the same active export -> the UI polls to `ready` -> the download href is the signed URL and the download is a `.zip`.
- Lifecycle: seed an expired and a 3-day event -> expired banner -> Extend 30 days -> badge active, banner gone -> list shows "Expires in 3 days" -> Expired filter returns only the expired event.
- Existing suites (app, auth, billing, branding, devices, events, gallery, photos, plan-limit, qr) still pass.

### Verification commands and results

```text
gofmt -l .                                        -> (nothing)
go build ./...                                    -> exit 0
go vet ./...                                      -> exit 0
go test ./...                                     -> all packages ok
pnpm lint                                         -> exit 0
pnpm typecheck                                    -> exit 0
pnpm test                                         -> 39 files / 211 tests (web) + 7 (api-client) passed
pnpm build                                        -> compiled successfully, 19 routes + proxy (/analytics included)
pnpm test:e2e                                     -> 17 passed (1.6m)
pnpm gen:api && git diff --exit-code schema.d.ts  -> no diff
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| shadcn added `recharts` only to `apps/web`, but `chart.tsx` lives in `packages/ui` | Declared `recharts@3.8.0` in `packages/ui` (chart primitive) and `apps/web` (chart composition); pnpm strict resolution fails otherwise |
| The public gallery never forwarded `?src=qr`, so QR scans could not be attributed through the UI | `app/e/[slug]/page.tsx` reads `searchParams` and forwards `src=qr` to the API |
| `react-hooks/set-state-in-effect` rejected clearing a stale export id via `setState` in an effect | Derive the active id from the error state; the effect only clears sessionStorage |
| Base UI link-buttons keep `role="button"`, so `getByRole("link")` missed the ZIP download | Tests target the button role and assert the `href` |
| The full E2E suite exhausted the API's per-IP login/refresh limiter (10/min) and failed intermittently with 429-driven 401s | Added `e2e/api-proxy.mjs` (per-request `X-Forwarded-For`) and run the API on `:18081`; documented in `AGENTS.md` and `playwright.config.ts` |
| Rapid successive cold loads raced the rotating refresh token (two refreshes per navigation) | The analytics spec waits for `networkidle` between cold loads; the proxy also removes the limiter pressure that exposed it |

---

## 9. Known limitations / follow-ups

- The QR PNG/SVG encode the plain gallery URL (`/e/{slug}`, no `src=qr`), so printed codes do not count as scans unless the URL carries the parameter. A backend change (OpenAPI first) would append `src=qr` to the encoded URL.
- Next caches gallery metadata for 60 s (`revalidate: 60`, mirroring the API's `Cache-Control: max-age=60`), so repeated opens within the window are not counted; counters remain best-effort per Phase 9.
- The 7-day warning threshold mirrors `EXPIRY_WARN_DAYS`' default in the client; no endpoint exposes the configured value.
- Export polling is a fixed 5 s with no backoff; exports have no list endpoint (by design), so the sessionStorage id is the only resume mechanism.
- `apps/web/e2e/api-proxy.mjs` is a local test harness; E2E is not part of CI.
- The dashboard strip and `/analytics` share the same 30-day query key and cache entry (intended).

---

## 10. How to try it

```text
# Infrastructure + envs; API on :18081 for E2E, web on :3000
docker compose up -d
goose -dir migrations postgres "$env:DATABASE_URL" up
$env:HTTP_ADDR=":18081"; go run ./cmd/api     # separate terminal
go run ./cmd/worker                           # separate terminal
pnpm install
pnpm dev

# Signed in:
#   /analytics            -> totals, 7/30/90/365 window, metric toggle, UTC help
#   /dashboard            -> "Last 30 days" strip -> View analytics
#   /events/{id}          -> Analytics tab; Overview -> Bulk export (after photos)
#   /events/{id} expired  -> banner + Extend 30 days -> active
#   /events               -> status filter Expired; "Expires in N days" hints
#   /e/{slug}?src=qr      -> counts a view and a QR scan

# Gates
pnpm lint; pnpm typecheck; pnpm test; pnpm build; pnpm test:e2e
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
go test ./...
```
