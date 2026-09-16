# Phase F6 — Analytics, Exports & Lifecycle

> **Status: COMPLETE.** See the [Phase F6 Report](Phase F6 - Report.md) for what was built and how it was verified. Backend Phase 9 is complete; this phase consumes its APIs with no backend changes.

**Goal:** Operators can see how their galleries perform, hand clients a ZIP of an event's photos, and manage event expiry — completing the last operator surface against the Phase 9 APIs.

**Depends on:** Phase F0 (env, api-client, shell); Phase F1 (session, `apiCall`); Phase F4 (signed-URL/download rules); Phase 9 (analytics, exports, extend, expiry — complete). No backend changes required.

**Exit criteria:** An operator opens Analytics and sees all-time totals plus a daily series for a selectable 7/30/90/365-day window, account-wide and per event; requests a ZIP export for an event, watches it reach `ready`, and downloads it from the signed URL; extends an event's expiry (including reactivating an expired event); and sees expired / expiring-soon state in the events list and event detail, with archived events offering no extend action.

---

## 1. Features

### 1.1 Analytics

- `/analytics` page (sidebar: Analytics, `ChartColumn`, between Dashboard and Events):
  - Window selector 7 / 30 / 90 / 365 days (default 30, the API default). The window is part of the query key; custom day counts are not offered (the API accepts 1–365) so the chart stays readable and cacheable.
  - Totals row from `GET /account/analytics`: gallery views, unique visitors, downloads, QR scans, plus all-time `eventCount` and `photoCount`.
  - Daily chart for the selected window with a metric toggle (views by default; visitors / downloads / QR scans switchable) — one series at a time.
  - Empty state when the window has no activity (all-time totals may still be non-zero).
  - Help text: counters are UTC days; `uniqueVisitors` sums daily uniques so a returning visitor counts again; downloads only accrue when original downloads are enabled.
- Event detail gains an **Analytics** tab (between Overview and Photos) using `GET /events/{id}/analytics` — same totals + chart components, event-scoped, showing `photoCount`.
- Dashboard adds a compact "Last 30 days" strip (views, visitors, downloads, QR scans) linking to `/analytics`.

### 1.2 Bulk ZIP exports

- `ExportCard` in the event overview:
  - "Export all photos (ZIP)" → `POST /events/{id}/exports` (202). Disabled with an explanatory hint when `event.photoCount === 0` (the API answers 422 `VALIDATION_ERROR` otherwise).
  - The returned export id is persisted per event in `sessionStorage` so polling can resume after a reload — the API has **no export list endpoint**.
  - Poll `GET /events/{id}/exports/{exportID}` every 5 s while `pending|processing`; stop on `ready|failed|expired` and on unmount.
  - `ready`: show `fileSize` + "expires in {relative time}" and a Download action that navigates to `downloadUrl` (signed URL, browser → R2 directly; the ZIP content type triggers the download).
  - `failed`: show the failure and a "Generate new export" action (a fresh POST; failed exports never retry).
  - `expired`: show expired state + "Generate new export".
  - POST dedupe: while an export is active the API returns the existing one; the UI never creates duplicates.
- The export object is cached under `eventKeys.export(eventId, exportId)`.

### 1.3 Lifecycle

- **Expiry visibility:** the event Details card shows "Expires {datetime}" with an `ExpiryLabel` ("expires in N days" / "expired N days ago" / "never"); warning tone at ≤ 7 days (mirrors `EXPIRY_WARN_DAYS`), destructive tone when expired. Events list cards get the same hint.
- **Expired banner** on event detail: "This event expired on {date}. Its gallery is no longer available. Extend to reactivate." with an Extend action. The public gallery 404s on/past `expiresAt` (Phase 9).
- **Extend dialog:** preset chips 7 / 30 / 90 days + custom 1–3650; submit `POST /events/{id}/extend` `{days}`; success updates the event cache (an expired event becomes active) and invalidates the list; toast shows the new expiry. Hidden for `archived` events (the API answers 409 `INVALID_STATUS_TRANSITION`); copy explains the extension runs from `max(now, expiresAt)`.
- **Events list:** add `expired` to the status filter (currently missing) and an "Expiring soon" hint on cards whose `expiresAt` is within 7 days.
- **Danger zone copy:** state the grace period explicitly — deleting hides the event immediately, photos stay recoverable for `EVENT_PURGE_GRACE_DAYS` (30 default), then the worker purges DB rows and R2 objects permanently; there is no restore endpoint.
- Expired events keep their other panels (Photos, Share/QR, Settings): the API does not block them and F6 adds no client-side business rule (Conventions §10). The banner and Extend are the guidance.

### 1.4 Shell & guard

- Sidebar entry Analytics (`ChartColumn`).
- `proxy.ts`: add `/analytics` to `PROTECTED_PREFIXES` and the matcher.

---

## 2. Backend contract (no changes required)

Phase 9 endpoints and the generated client already exist; quirks the UI must respect:

- **Analytics:** `?days=1..365` (default 30) is an inclusive UTC window ending today (`days=1` = today only); `daily` contains only days with activity — zero-fill to a contiguous window before charting; `uniqueVisitors` is the sum of exact daily uniques; `photoCount`/`eventCount` are all-time, never windowed. Counters are recorded best-effort on public reads: a view per successful `GET /public/events/{slug}` (public or unlocked), `qrScans` when that request carries `?src=qr`, `downloads` when an `original` URL is issued (only when downloads and originals are enabled). Bad `days` → 422 `VALIDATION_ERROR`; other tenants' events → 404 `EVENT_NOT_FOUND`.
- **Exports:** POST is create-or-return-active (dedupes `pending|processing`) and returns 202; there is **no list endpoint**; `downloadUrl` exists only while `ready` and unexpired and is a short-lived signed URL (`min(remaining life, EXPORT_TTL)`); `expiresAt` is `EXPORT_TTL` after the archive is built (24 h default); `failed` never retries; POST on an event with no photos → 422 `VALIDATION_ERROR`; unknown/other-tenant export → 404 `EXPORT_NOT_FOUND`.
- **Extend:** `{days}` 1–3650; extends from `max(now, expiresAt)`; reactivates `expired`; `archived` → 409 `INVALID_STATUS_TRANSITION`; a `null` `expiresAt` (never expires) becomes a real date after an extend.
- **Statuses:** `expired` is a first-class `EventStatus` (the list filter accepts it); transitions are `upcoming|active|completed → expired`, `active|completed → archived`, `expired → active` (extend). Delete is soft; purge after the grace period is irreversible and has no API.
- **Warnings:** expiry warnings are worker-log-only (no email yet), so the UI derives "expires soon" from `expiresAt`; keep the 7-day threshold aligned with `EXPIRY_WARN_DAYS`.
- **Uploads:** the API does not reject uploads to expired events. F6 surfaces the banner and Extend instead of adding a client-side block; server-side enforcement is a backend follow-up if wanted.
- No `api/openapi.yaml` change; `pnpm gen:api` must produce no diff.
- **Error codes to map:** `EVENT_NOT_FOUND`, `EXPORT_NOT_FOUND`, `VALIDATION_ERROR`, `INVALID_STATUS_TRANSITION`, `RATE_LIMITED`.
- **New dependency:** the shadcn `chart` primitive (`pnpm shadcn add chart -c apps/web`), which lands `chart.tsx` in `packages/ui` and brings `recharts`. It is the only new runtime dependency in this phase; all charts go through `ChartContainer`.

---

## 3. API surface consumed

```http
GET  /api/v1/account/analytics?days=1..365
GET  /api/v1/events/{eventID}/analytics?days=1..365
POST /api/v1/events/{eventID}/exports
GET  /api/v1/events/{eventID}/exports/{exportID}
POST /api/v1/events/{eventID}/extend
GET  /api/v1/events?status=expired   # existing list, newly surfaced
GET  /api/v1/events/{eventID}        # existing; expiresAt/status
```

---

## 4. File map

```text
apps/web/src/app/(dashboard)/
  analytics/page.tsx                    # server wrapper -> AnalyticsPanel
apps/web/src/features/analytics/
  api.ts keys.ts schema.ts errors.ts format.ts
  analytics-panel.tsx                   # /analytics: window select, totals, chart
  analytics-totals.tsx                  # 4 counters + event/photo counts
  daily-chart.tsx                       # ChartContainer + recharts, zero-filled series
  window-select.tsx                     # 7 / 30 / 90 / 365
apps/web/src/features/events/
  analytics-tab.tsx                     # event-scoped; reuses analytics/* components
  extend-expiry-dialog.tsx
  expiry-label.tsx                      # "expires in N days" / "expired N days ago"
  export-card.tsx                       # create-or-resume, poll, download
  api.ts keys.ts errors.ts              # extend/export/analytics hooks + keys
  event-detail.tsx                      # Analytics tab, expired banner, extend, export card
  events-list.tsx                       # expired filter + expiring-soon hint
  danger-zone.tsx                       # grace-period copy
apps/web/src/app/(dashboard)/dashboard/page.tsx   # 30-day analytics strip
apps/web/src/components/app-sidebar.tsx           # Analytics nav entry
apps/web/src/proxy.ts                             # /analytics protected + matcher
packages/ui/src/components/chart.tsx              # shadcn chart primitive
```

---

## 5. Data and state rules

- All calls go through `apiCall` from `lib/auth/api.ts`; never hand-write API types — use the generated operations.
- Query keys are colocated: `analyticsKeys.account(days)`, `analyticsKeys.event(eventId, days)`, `eventKeys.export(eventId, exportId)`. The window is part of the key.
- Export polling uses `refetchInterval` returning `false` for terminal statuses (`ready|failed|expired`); the query is enabled only while an export id exists.
- Resume across reloads: `sessionStorage["cpd:export:{eventId}"]` stores only the export id — written on POST, read on mount to seed the polling query, replaced when a new export is requested. Never store the signed `downloadUrl`.
- Extend mutation updates `eventKeys.detail` and invalidates `eventKeys.lists()`; the event may change status (`expired → active`) and date.
- Downloads navigate to `downloadUrl`; bytes are never fetched through Next or the Go API.
- Dates: display in the operator's local timezone; label analytics as UTC days in the UI copy.
- No business rules in the frontend (Conventions §10): range limits (1–365, 1–3650) are mirrored as form constraints but the API stays authoritative; map 422 to field errors.

---

## 6. UX rules

- Charts: one metric at a time via a toggle; the series is zero-filled; the tooltip shows the UTC date and value; include a text summary (`aria-label` or visually hidden list) for screen readers since charts are not natively accessible.
- Numbers use `Intl.NumberFormat`; "No activity yet" for empty charts and "—" for loading counters, never a flash of `0`.
- Expiry: warning tone within 7 days, destructive tone when expired; the Extend dialog offers presets and explains the `max(now, expiresAt)` rule when already expired; archived events get no Extend affordance.
- Exports: the primary action is disabled with a hint when there are no photos; polling shows a progress state; ready shows size and expiry; failure/expiry offer "Generate new export"; downloading opens the signed URL directly.
- Errors: feature-local `errors.ts` maps codes to copy; never render raw server messages. `INVALID_STATUS_TRANSITION` on extend (archived) explains "Archived events cannot be extended."
- Accessibility: dialogs are keyboard operable and trap focus; charts have text alternatives; status changes announce via toast.

---

## 7. Test Plan

**Unit (Vitest + RTL)**

- [ ] analytics: sparse `daily` arrays are zero-filled to the window; window validation mirrors 1–365; UTC date labels; total formatting; `EVENT_NOT_FOUND` mapping.
- [ ] window selector changes the query key and refetches; default is 30.
- [ ] export: disabled when `photoCount === 0`; POST persists the export id; polling interval stops on `ready|failed|expired`; resume from `sessionStorage`; download uses `downloadUrl`; expiry countdown.
- [ ] extend: preset payloads; custom 1–3650 validation; archived hides the action; success updates status (`expired → active`) and invalidates lists.
- [ ] expiry label: "expires in N days" at exactly 7 days, "expired N days ago", "never" for null.
- [ ] events list: expired filter option present; expiring-soon hint only inside 7 days.
- [ ] chart: renders zero-filled series and switches metrics.

**E2E (Playwright, live API; skip when `/readyz` is unreachable)**

- [ ] Analytics: create an event → open `/e/{slug}` (and once with `?src=qr`) → `/analytics` shows views and QR scans ≥ 1; the event Analytics tab shows the same event's counters.
- [ ] Export: seed photos via the API (MinIO + worker required) → Export ZIP → poll to `ready` → the download link points at the signed URL; a second request while active returns the same export.
- [ ] Lifecycle: create an event with `status: "expired"` (or a backdated `expiresAt`) → detail shows the expired banner → Extend 30 days → status becomes active and the date updates.
- [ ] Expiring soon: create an event with `expiresAt` in 3 days → the list card shows "Expires in 3 days".
- [ ] Events list: filter by Expired returns only expired events.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # against live Go API + Postgres (MinIO + worker for the export seeding)
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
go test ./...
```

---

## 8. Definition of Done

- [ ] All master DoD items that apply; no backend changes (if a contract gap is found, OpenAPI lands first).
- [ ] Analytics windows match the API (1–365); sparse days zero-filled; UTC semantics explained in the UI.
- [ ] Export polling stops on every terminal state; the signed `downloadUrl` is never persisted, logged, or proxied; only the export id survives a reload.
- [ ] Expired events show the banner + Extend; archived events have no Extend affordance; the events list offers the expired filter.
- [ ] No client-side business rules added; the API remains authoritative for expiry, limits, and lifecycle.
- [ ] The only new dependency is `recharts` via the shadcn `chart` primitive in `packages/ui`.
- [ ] `pnpm test:e2e` covers analytics, export ready-download, and extend reactivation.
- [ ] AGENTS.md updated if new commands or env vars are introduced (expected: none).
- [ ] `doc/Implementation Plan.md` frontend track references this doc.
- [ ] Completion report written from the template.
