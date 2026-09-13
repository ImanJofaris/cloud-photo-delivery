# Phase F5 — Devices, Branding & Billing — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

---

## 1. Summary

Phase F5 completes the operator surface: devices and one-time API keys, tenant branding with asset uploads and live preview, event QR codes fetched as bearer-authenticated blobs, and plan/subscription/invoice management against the Phase 6–8 APIs. It also closed the dashboard auth gap (only `/dashboard` was protected before) and wired every `PLAN_LIMIT_REACHED` dead-end to `/billing`. No backend or OpenAPI changes were needed; `pnpm gen:api` is diff-stable.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| An operator creates a device, saves the one-time key, rotates and revokes it | PASS | `features/devices/*`; E2E `devices.spec.ts` create → reveal → rotate → revoke; revoked badge persists after reload |
| Saves branding and sees it on the public gallery | PASS | `features/branding/*`; E2E `branding.spec.ts` saves name/color/email and asserts them on `/e/{slug}` |
| Opens an event's QR code, downloads PNG/SVG, and prints it | PASS | `features/events/qr-dialog.tsx` object-URL preview + download/print; E2E `qr.spec.ts` asserts `-qr.png` / `-qr.svg` downloads and copy link |
| Views the effective plan and usage, subscribes, upgrades/downgrades, cancels/resumes, and pages through invoices | PASS | `features/billing/*`; E2E `billing.spec.ts` subscribe (manual) → cancel → resume; unit tests cover action selection and invoice paging |
| Every dashboard route redirects to login when signed out | PASS | `proxy.ts` protects `/dashboard /events /account /devices /branding /billing`; `proxy.test.ts` covers each prefix and lookalikes |
| `PLAN_LIMIT_REACHED` (402) links to `/billing` in events and uploads | PASS | `event-form.tsx` "View plans" link; upload queue row link; E2E `plan-limit.spec.ts` drives a real 402 and clicks through to `/billing` |

---

## 3. What was built

### 3.1 Session-aware binary calls (`apiBlobCall`)

- `apps/web/src/lib/auth/api.ts` — `apiCall` and the new `apiBlobCall` share a `withRefreshRetry` helper: 401 → single-flight `refreshSession()` → retry the original call once; `apiBlobCall` maps envelope errors to `ApiError` and returns a `Blob`.
- `resetApiClientForTests()` resets the lazily-created openapi-fetch client so tests can swap `fetch` between cases.
- `apps/web/src/lib/hooks/use-object-url.ts` — shared object-URL hook (create on blob change, revoke on change/unmount).

### 3.2 Devices (`features/devices/`)

- `api.ts` / `keys.ts` / `errors.ts` / `schema.ts` — list/create/rename/rotate/revoke hooks (create and rotate return `DeviceWithKey`), code-mapped errors, 120-char name schema and request builder.
- `devices-panel.tsx` — table (name, key prefix, resolved event, status, last used, created); revoked rows muted with a Revoked badge and disabled actions; empty state documents event scoping and the 100 uploads/min per-device limit; event names resolve from a cached `useEventOptions()` fetch with an "Unknown event" fallback.
- `device-form-dialog.tsx` — name + active-event select (`No event (unscoped)` sends no `assignedEventId`).
- `device-key-dialog.tsx` — one-time key, masked until reveal, copy + `X-Api-Key` example, not dismissible by Esc/overlay; the only exit is "I've saved it". The key lives in component state only.
- `rename-device-dialog.tsx`, `rotate-key-dialog.tsx`, `revoke-device-dialog.tsx` — confirmations; rename copy explains that scope changes require a new device.

### 3.3 Branding & QR (`features/branding/`, `features/events/qr-dialog.tsx`)

- `branding/schema.ts` — PNG/JPEG/WebP ≤ 5 MB validation, lowercase `#rrggbb` zod schema, `toBrandingPatch` partial diff (omitted fields unchanged, empty string clears, asset keys only when staged).
- `branding/asset-upload.tsx` — `uploadBrandingAsset` runs presign → XHR PUT with progress (reusing `features/photos/transport.ts`) and re-presigns once on an expired PUT (400/403); `AssetUploadField` shows progress/retry/remove.
- `branding/api.ts` / `keys.ts` — `GET/PATCH /account/branding`, presign mutation; PATCH success seeds the query cache.
- `branding-form.tsx` — RHF form with color pickers bound to text fields, staged asset previews via object URLs, clear confirmations, "Save again" after a failed save, live `BrandingPreview` of the gallery header.
- `features/events/qr-dialog.tsx` — fetches PNG (size 1024) and SVG through `apiBlobCall`, renders the PNG object URL, Copy link, Download PNG/SVG (`{event-name}-qr.png|svg`), and Print via a synchronously-opened self-contained window with inline SVG + "Scan to view photos".
- `features/events/share-panel.tsx` — now takes `eventId`, `eventName`, `slug` and hosts the QR dialog.

### 3.4 Billing (`features/billing/`)

- `api.ts` / `keys.ts` — plans, subscription (`staleTime: 0`, `refetchOnWindowFocus: true` for lazy expiry), infinite invoices (limit 20), and subscribe/upgrade/downgrade/cancel/resume mutations that invalidate subscription **and** invoices.
- `format.ts` — MYR formatting, `0` → "Unlimited", usable-status predicate, price comparison action selection (`Subscribe`/`Upgrade`/`Downgrade`/`Current`), yearly = 12× monthly, usage percent.
- `errors.ts` — maps `PLAN_NOT_FOUND`, `SUBSCRIPTION_NOT_FOUND`, `SUBSCRIPTION_ALREADY_ACTIVE`, `INVALID_STATUS_TRANSITION`, `PLAN_LIMIT_REACHED`, `VALIDATION_ERROR`; never leaks server messages.
- `subscription-card.tsx` / `usage-meters.tsx` — effective plan + status badge, interval, renewal/cancellation date, Resume/Cancel, over-limit highlighting.
- `plan-grid.tsx` / `plan-card.tsx` — interval toggle on subscribe only, current plan highlighted, one action per card with an accessible label (e.g. "Subscribe to Starter").
- `change-plan-dialog.tsx` — subscribe/upgrade/downgrade/cancel/resume copy and confirmations; soft-downgrade explanation; `openCheckout` only opens `http(s)` URLs and shows the offline billing note (with the invoice pointer) for `manual://` — it never navigates there.
- `invoices-table.tsx` — amount, status badge, issued/paid dates, cursor "Load more".
- `billing-panel.tsx` — composition and dialog state.

### 3.5 Shell, navigation, and limit surfacing

- `components/app-sidebar.tsx` — Devices/Branding/Billing entries (`Camera`/`Palette`/`CreditCard`) and `isNavActive` nested-route matching.
- `proxy.ts` + `proxy.test.ts` — six protected prefixes and all six added to the matcher; `next=` preserved.
- `app/(dashboard)/dashboard/page.tsx` — stale F2 placeholder replaced with a plan/usage summary (active events, storage) linking to `/billing`.
- `lib/api-errors.ts` — shared `isPlanLimitReached`; `event-form.tsx` renders a destructive alert with a "View plans" link, and the upload queue renders the same link on `PLAN_LIMIT_REACHED` failures (`uploader.ts` now records `errorCode`).

---

## 4. Database changes

None. No migrations.

---

## 5. API surface added

No new endpoints and no OpenAPI changes. `pnpm gen:api` produces no `schema.d.ts` diff. Consumed:

```http
POST   /api/v1/devices
GET    /api/v1/devices
PATCH  /api/v1/devices/{deviceID}
DELETE /api/v1/devices/{deviceID}
POST   /api/v1/devices/{deviceID}/rotate

GET    /api/v1/account/branding
PATCH  /api/v1/account/branding
POST   /api/v1/account/branding/assets

GET    /api/v1/events/{eventID}/url
GET    /api/v1/events/{eventID}/qr.png?size=
GET    /api/v1/events/{eventID}/qr.svg

GET    /api/v1/billing/plans
GET    /api/v1/billing/subscription
POST   /api/v1/billing/subscribe
POST   /api/v1/billing/upgrade
POST   /api/v1/billing/downgrade
POST   /api/v1/billing/cancel
POST   /api/v1/billing/resume
GET    /api/v1/billing/invoices?cursor=&limit=
```

---

## 6. Files created / modified

```text
apps/web/src/app/(dashboard)/devices/page.tsx
apps/web/src/app/(dashboard)/branding/page.tsx
apps/web/src/app/(dashboard)/billing/page.tsx
apps/web/src/features/devices/{api,keys,errors,schema,format}.ts
apps/web/src/features/devices/{devices-panel,device-form-dialog,device-key-dialog,rename-device-dialog,rotate-key-dialog,revoke-device-dialog}.tsx
apps/web/src/features/branding/{api,keys,schema}.ts
apps/web/src/features/branding/{branding-form,branding-preview,asset-upload}.tsx
apps/web/src/features/billing/{api,keys,errors,format}.ts
apps/web/src/features/billing/{billing-panel,subscription-card,usage-meters,plan-grid,plan-card,change-plan-dialog,invoices-table}.tsx
apps/web/src/features/events/qr-dialog.tsx
apps/web/src/lib/api-errors.ts
apps/web/src/lib/hooks/use-object-url.ts
apps/web/src/components/app-sidebar.test.ts
apps/web/src/proxy.test.ts
apps/web/src/lib/auth/api.test.ts
apps/web/src/features/devices/{schema,format}.test.ts
apps/web/src/features/devices/{device-key-dialog,devices-panel}.test.tsx
apps/web/src/features/branding/{schema,asset-upload}.test.ts
apps/web/src/features/branding/branding-form.test.tsx
apps/web/src/features/billing/{format,invoices-table}.test.tsx
apps/web/src/features/billing/change-plan-dialog.test.tsx
apps/web/src/features/events/qr-dialog.test.tsx
apps/web/e2e/{devices,branding,billing,qr,plan-limit}.spec.ts
packages/ui/src/components/{table,alert-dialog}.tsx
```

Modified: `apps/web/src/lib/auth/api.ts`, `apps/web/src/proxy.ts`, `apps/web/src/components/app-sidebar.tsx`, `apps/web/src/app/(dashboard)/dashboard/page.tsx`, `apps/web/src/features/events/{api,keys,format,event-detail,event-form,share-panel}.ts(x)`, `apps/web/src/features/photos/{errors,uploader,upload-queue}.tsx`, `apps/web/src/features/photos/upload-queue.test.tsx`, `apps/web/src/features/events/format.test.ts`, `apps/web/vitest.setup.ts`, `apps/web/playwright.config.ts`, `doc/Implementation Plan.md`, `doc/phases/Phase F5 - Devices, Branding and Billing.md`, `doc/phases/Phase 6/7/8 - Report.md`.

---

## 7. Tests

### Unit (Vitest + RTL)

- `apiBlobCall`: returns blobs, refreshes once on 401 and retries with the new token, maps envelope errors, throws on non-envelope failures.
- `proxy`: each protected prefix redirects to `/login?next=…`, signed-in auth pages redirect to `/dashboard`, `/e/*` stays public; `isProtectedPath` rejects lookalikes.
- Sidebar `isNavActive` exact/nested/lookalike matching.
- Devices: schemas and request builders; one-time key masking/reveal/copy/ack; create flow ends in the key dialog; revoked rows disable actions; unknown-event fallback; rotate and revoke confirmations.
- Branding: zod rejects alpha colors and invalid email/URL; asset type/5 MB validation; `toBrandingPatch` omits unchanged fields, normalizes colors, clears with empty strings, includes staged keys; upload order presign → PUT, re-presign on expiry, no re-presign on 500; form sends a partial PATCH and blocks invalid email.
- Billing: `0` → Unlimited, MYR formatting, usage caps; Subscribe/Upgrade/Downgrade/Current selection; usable-status predicate; `manual://` shows the offline note and never calls `window.open`; hosted checkout opens; error mapping never leaks server text; invoice badges/amounts/empty/paging.
- QR: blob preview rendered, object URLs revoked on unmount, PNG/SVG download filenames differ, error state, print document escapes the event name and embeds the inline SVG.
- Uploads: plan-limit failure renders the `/billing` link.

### Integration (`//go:build integration`)

Not run — no backend changes in F5.

### E2E (Playwright, live API, workers = 1)

- Devices: create → one-time key (masked → revealed) → rotate → revoke → revoked persists after reload.
- Branding: save name/color/email → open `/e/{slug}` → branding renders.
- Billing: subscribe to Starter (manual, offline note) → active card + usage → cancel → resume.
- QR: blob image renders, PNG/SVG downloads, copy link.
- Plan limit: a real 402 renders the message with a working `/billing` link.
- Existing suites (app, auth, events, gallery, photos) still pass.

### Verification commands and results

```text
gofmt -l .                                        -> (nothing)
go build ./...                                    -> exit 0
go vet ./...                                      -> exit 0
go test ./...                                     -> all packages ok
pnpm lint                                         -> exit 0 (no warnings)
pnpm typecheck                                    -> exit 0
pnpm test                                         -> 29 files / 159 tests (web) + 7 (api-client) passed
pnpm build                                        -> compiled successfully, 18 routes + proxy
pnpm test:e2e                                     -> 14 passed (1.2m)
pnpm gen:api && git diff --exit-code schema.d.ts  -> no diff
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| `proxy.ts` only guarded `/dashboard`, leaving `/events`, `/account`, and the new F5 routes public after sign-out | `PROTECTED_PREFIXES` + matcher expanded; unit tests per prefix |
| Sidebar marked only exact matches active, so nested routes lost their highlight | `isNavActive` matches `href` and `href + "/"` |
| QR/branding binaries cannot go through `apiCall` (envelope-only) | `apiBlobCall` with the same 401 refresh + retry-once semantics; blobs render via object URLs |
| The API auth limiter (30/min, burst 10 per IP) broke the E2E suite once it grew to 14 tests under parallel workers | `playwright.config.ts` runs `workers: 1` locally (CI does not run E2E); documented in the config |
| The F2 create-event form never sends `status: "active"`, so the active-event 402 cannot be reached through normal UI clicks | The plan-limit E2E marks the UI create request active via `page.route` to exercise the real API 402 and the real error UI; noted as a follow-up (an activate/transition UI would make it reachable) |
| Base UI `Button render={<Link/>}` keeps `role="button"` (it is an anchor) | E2E targets the button role and asserts the `href` |

---

## 9. Known limitations / follow-ups

- `PATCH /account/me` OpenAPI drift and the account edit form remain unbuilt (backlog).
- Event create/transition does not expose `active`; the events 402 link is wired but only reachable when the request declares an active event (e.g. via API). A status transition UI would close this.
- The F4 viewer still lacks end-of-page advance; original-filename downloads and watermark worker support remain backlog.
- Branding asset replacement leaves orphaned objects until the Phase 9 cleanup job; the form keeps a failed PATCH's preview so "Save again" can finish.
- `manual://` checkout stays offline by design; polling/hosted gateways arrive with a real provider adapter.
- Phase F6 (analytics/exports/lifecycle) stays blocked on Phase 9.

---

## 10. How to try it

```text
# Infrastructure + envs (see Phase 8 report); API on :18080, web on :3000
pnpm install
pnpm dev

# Signed out, open http://localhost:3000/devices → redirected to /login?next=/devices
# Sign up, then:
#   /devices  → Add device → save the one-time key → rotate → revoke
#   /branding → set business name + colors + contact → Save → open /e/{slug}
#   /events/{id} → Share → QR code → reveal/download/print
#   /billing  → Subscribe to Starter (offline note) → Cancel → Resume
#   /events/new on Free with an active event → 402 alert with "View plans"

# Gates
pnpm lint; pnpm typecheck; pnpm test; pnpm build; pnpm test:e2e
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
```
