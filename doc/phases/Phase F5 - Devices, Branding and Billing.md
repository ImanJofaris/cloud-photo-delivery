# Phase F5 — Devices, Branding & Billing

> **Status: COMPLETE.** See the [Phase F5 Report](Phase F5 - Report.md) for what was built and how it was verified. Backend phases 6–8 are complete; this phase consumes them with no API changes.

**Goal:** Operators can register and manage photobooth devices and their API keys, configure tenant branding (with QR codes for events), and manage plans, usage, and invoices — completing the remaining operator surfaces against the Phase 6–8 APIs.

**Depends on:** Phase F0 (env, api-client, shell); Phase F1 (session, `apiCall`); Phase 6 (devices API); Phase 7 (branding + QR API); Phase 8 (billing API). No backend changes required.

**Exit criteria:** An operator creates a device, saves the one-time key, rotates and revokes it; saves branding and sees it on the public gallery; opens an event's QR code, downloads PNG/SVG, and prints it; views the effective plan and usage, subscribes, upgrades/downgrades, cancels/resumes, and pages through invoices; every dashboard route redirects to login when signed out.

---

## 1. Features

### 1.1 Devices & API keys (`/devices`)

- List all devices (newest first, unpaginated) in a table: name, `keyPrefix`, assigned event, status (Active/Revoked), last used, created.
- Create dialog: name (≤ 120) + optional assigned event (active events only; "No event (unscoped)" = `assignedEventId: null`).
- **One-time key dialog** after create and rotate: masked by default, reveal + copy, explicit warning that the key cannot be shown again, the exact header (`X-Api-Key`) and an example upload call.
- Rename dialog (`PATCH`, name only). Assignment changes are not supported by the API — copy explains that changing scope means creating a new device.
- Rotate: confirm dialog → new one-time key dialog (the old key dies immediately).
- Revoke: confirm dialog → soft revoke; the row stays with a Revoked badge for audit and all actions disable. No hard delete anywhere.
- Empty state explaining device-key uploads are scoped to the assigned event and limited to 100 uploads/min per device.
- Resolve assigned event names from the cached events list; fall back to a muted "Unknown event" instead of fetching per row.

### 1.2 Branding & QR

- `/branding` page: business name, logo, profile image, primary/secondary colors, contact email/phone, website — with a live preview of the gallery header.
- Asset upload flow: `POST /account/branding/assets` → browser `PUT` directly to R2 with progress (reuse the XHR transport from `features/photos/transport.ts`) → `PATCH /account/branding` with the returned `storageKey`.
- Client-side asset validation (the server does not inspect bytes): PNG/JPEG/WebP, max 5 MB; show the upload progress and a retry on failure.
- Clear logo/profile image sends the empty string (verified: `logoKey: ""` clears) behind a confirm.
- Colors are `#rrggbb` only (no alpha): `<input type="color">` bound to a text field, normalized to lowercase.
- One partial `PATCH` sends only changed fields; omitted fields stay unchanged. Preview falls back to defaults for unset fields.
- **Event QR** in the event detail Share panel: fetch `GET /events/{id}/url` and the QR image **as a blob through the authenticated client** (endpoints are bearer-authenticated, and responses are binary, not enveloped), render via an object URL, and offer Copy link, Download PNG (1024), Download SVG, and Print.
- Print opens a self-contained window (opened synchronously, populated with the inline SVG, event name, and "Scan to view photos") and calls `print()` — no global print CSS.

### 1.3 Billing (`/billing`)

- Current plan card: effective plan (Free when `subscription` is null), status badge, interval, current period end, `cancelAtPeriodEnd` notice with a Resume action, and usage meters (active events, storage bytes; `0` = Unlimited).
- Plan grid from `GET /billing/plans`: MYR pricing, limits with unlimited semantics, current plan highlighted, and one action per card:
  - **Subscribe** when there is no usable subscription (paid plans only; Free is shown as current, not subscriable).
  - **Upgrade** (higher `priceCents`) or **Downgrade** (lower) when a usable subscription exists.
  - Interval toggle (month/year) on subscribe only; upgrades/downgrades keep the existing interval.
- Downgrade confirm explains the soft-downgrade rule: data is kept, new actions are blocked while over the target limits.
- Cancel confirm; while `cancelAtPeriodEnd`, show "Cancels on {date}" and Resume instead of Cancel.
- Manual checkout: the subscription activates immediately; if `checkout.url` is `http(s)` open it, otherwise (current `manual://` provider) show an offline billing note and the open invoice — never navigate to `manual://`.
- Invoices: cursor-paginated list (limit 20, Load more) with amount, status badge (open/paid/void), issued/paid dates.
- Lazy expiry: refetch the subscription on mount and window focus; after `currentPeriodEnd` the API flips it to expired on read and the UI falls back to Free.
- **Upgrade path for limits:** `PLAN_LIMIT_REACHED` (402) in event create and upload flows gets an actionable link/button to `/billing` instead of a dead-end message.

### 1.4 Shell, navigation & auth guard

- Sidebar: add Devices (`Camera`), Branding (`Palette`), Billing (`CreditCard`); active state must match nested routes (`href + "/"` prefix), fixing the current exact-match behavior.
- Fix the auth gap: the dashboard route group serves `/events`, `/account`, etc., but `proxy.ts` only guards `/dashboard`. Protect all dashboard prefixes and add them to the matcher so signed-out users are redirected to `/login?next=`.
- Replace the stale dashboard placeholder ("Event management arrives in Phase F2") with a compact plan/usage summary linking to `/billing`.

---

## 2. Backend contract (no changes required)

Phases 6–8 already expose everything F5 needs; `packages/api-client` already contains the operations. Quirks the UI must respect:

- **Devices:** raw key returned exactly once (create/rotate); `PATCH` renames only; revoke is soft and revoked devices stay listed; `GET /devices` has no pagination; device auth is upload-only and throttled to 100/min per device (`429` + `Retry-After`).
- **Branding:** scope is per **tenant**, not per event. Omitted PATCH fields are unchanged; empty strings clear (including `logoKey`/`profileImageKey`). Colors must be lowercase `#rrggbb`. The server does not inspect uploaded bytes, so the client owns type/size validation. Asset presigned PUTs expire with `SIGNED_URL_TTL` (5 min default).
- **QR:** `GET /events/{id}/url` returns `<PUBLIC_BASE_URL>/e/{slug}` (never signed); `qr.png`/`qr.svg` are bearer-authenticated binary responses with `Cache-Control: public, max-age=3600`; `size` clamps 64–2048, default 512.
- **Billing:** at most one non-terminal subscription; effective plan is Free when `subscription` is null or terminal. Subscribe with the `manual` provider activates immediately and returns an offline `manual://` checkout URL. Upgrade/downgrade is decided by `priceCents`; downgrade is soft. Cancel keeps the plan until `currentPeriodEnd`; Resume only when `cancelAtPeriodEnd`. Lazy expiry happens only on `GET /billing/subscription` — refetch after the period ends. Invoices are keyset-paginated (default 20, max 100). `0` always means unlimited.
- **Error codes to map:** `DEVICE_NOT_FOUND`, `INVALID_DEVICE_KEY`, `DEVICE_REVOKED`, `EVENT_NOT_FOUND`, `PLAN_LIMIT_REACHED` (402), `PLAN_NOT_FOUND`, `SUBSCRIPTION_NOT_FOUND`, `SUBSCRIPTION_ALREADY_ACTIVE`, `INVALID_STATUS_TRANSITION`, `VALIDATION_ERROR`, `RATE_LIMITED`.
- No `api/openapi.yaml` change; `pnpm gen:api` must produce no diff.

---

## 3. API surface consumed

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

`GET /events` (existing) is reused to populate the device assignment select and resolve names.

---

## 4. File map

```text
apps/web/src/app/(dashboard)/
  devices/page.tsx                    # server wrapper -> DevicesPanel
  branding/page.tsx                   # server wrapper -> BrandingForm
  billing/page.tsx                    # server wrapper -> BillingPanel
apps/web/src/features/devices/
  api.ts keys.ts errors.ts schema.ts
  devices-panel.tsx                   # table + actions + empty state
  device-form-dialog.tsx              # create (name + event select)
  device-key-dialog.tsx               # one-time reveal + copy
  rename-device-dialog.tsx
  rotate-key-dialog.tsx
  revoke-device-dialog.tsx
apps/web/src/features/branding/
  api.ts keys.ts schema.ts
  branding-form.tsx
  asset-upload.tsx                    # presign -> PUT (progress) -> key
  branding-preview.tsx                # gallery header preview
apps/web/src/features/billing/
  api.ts keys.ts errors.ts format.ts
  billing-panel.tsx
  subscription-card.tsx
  usage-meters.tsx
  plan-grid.tsx plan-card.tsx
  change-plan-dialog.tsx              # subscribe/upgrade/downgrade/cancel/resume
  invoices-table.tsx
apps/web/src/features/events/
  qr-dialog.tsx                       # blob QR preview, downloads, print
apps/web/src/lib/auth/api.ts          # add apiBlobCall (401 refresh + retry once)
apps/web/src/components/app-sidebar.tsx   # nav entries + nested active state
apps/web/src/proxy.ts                 # protected prefixes + matcher
apps/web/src/app/(dashboard)/dashboard/page.tsx  # plan/usage summary card
```

Reuse `features/photos/transport.ts` for the branding asset PUT (XHR progress + abort) and `features/events/format.ts` (`formatBytes`) for usage meters.

---

## 5. Data and state rules

- All calls go through `apiCall` from `lib/auth/api.ts`; QR binaries use a new `apiBlobCall` with identical 401 → single-flight refresh → retry-once semantics. Never hand-write API types; use the generated operations.
- Query keys are colocated: `deviceKeys.all|list`, `brandingKeys.all|detail`, `billingKeys.plans|subscription|invoices`.
- Mutations invalidate the narrowest key: device create/rotate/revoke → `deviceKeys.lists()`; branding save → `brandingKeys.detail()`; any billing mutation → subscription **and** invoices.
- Blob URLs: create only for the mounted dialog, revoke on unmount, never persist or log them. No `next/image` for QR or branding previews.
- Branding assets: bytes go browser → R2 directly; request a fresh presigned URL when one expires; PATCH only after a successful PUT; if the PATCH fails, keep the local preview and offer "Save again" (the orphaned object is acceptable until Phase 9 cleanup).
- Manual checkout is treated as offline: no `manual://` navigation, no polling; the subscription is already active on the response.
- Refresh subscription state after every mutation and on focus; do not cache billing data beyond the default query stale time.

---

## 6. UX rules

- Devices: revoked rows are muted with a Revoked badge and all actions disabled; the one-time key dialog cannot be dismissed without an explicit "I've saved it" acknowledgement; the key never appears in the URL, storage, or logs.
- Assignment select only lists active events; changing assignment is presented as "create a new device", never as a silent no-op.
- Branding: unset fields render as defaults in the preview; clear actions are confirmed; validation errors map from `VALIDATION_ERROR` to field-level messages.
- Billing: show effective plan and usage before any action; confirm dialogs for downgrade and cancel; explain the soft-downgrade rule in the dialog; `0` renders as "Unlimited", never "0".
- QR: dialog is event-scoped, previews the exact PNG/SVG the API returns, and downloads use the event name for the filename (`{event-name}-qr.png`).
- Errors: feature-local `errors.ts` maps codes to copy; never render raw server messages. `PLAN_LIMIT_REACHED` always offers the `/billing` action.
- Accessibility: dialogs trap focus and are keyboard operable; tables use real `<table>` semantics; uploads and color inputs are labelled; copy actions announce via toast.

---

## 7. Test Plan

**Unit (Vitest + RTL)**

- [ ] devices: create flow ends in the one-time key dialog; copy writes the key; rotate/revoke confirmations; revoked rows disable actions; `DEVICE_NOT_FOUND`/`DEVICE_REVOKED` mapping.
- [ ] devices: `updatedAt` formatting and assigned-event fallback ("Unknown event").
- [ ] branding: zod schema rejects alpha colors, invalid email/URL, oversized/unsupported assets; partial PATCH omits unchanged fields; empty string clears.
- [ ] branding: asset flow order presign → PUT → PATCH, PUT failure blocks PATCH, expiry re-requests a URL.
- [ ] QR: `apiBlobCall` refreshes once on 401 and retries; object URL revoked on unmount; download filenames.
- [ ] billing: usage formatting (`0` = Unlimited); action selection (Subscribe / Upgrade / Downgrade / Current / Resume); `manual://` shows the offline note and never navigates; invoice badges; error mapping (`SUBSCRIPTION_ALREADY_ACTIVE`, `INVALID_STATUS_TRANSITION`, `PLAN_LIMIT_REACHED`).
- [ ] shell: proxy redirects each protected prefix when the refresh cookie is absent; sidebar active state matches nested routes.

**E2E (Playwright, live API; skip when `/readyz` is unreachable)**

- [ ] Devices: create → one-time key visible → rotate → revoke; revoked badge persists after reload.
- [ ] Branding: save business name + color via the form → open `/e/{slug}` → branding renders.
- [ ] Billing: subscribe to Starter (manual) → card shows active + usage → cancel → resume (or upgrade to Pro).
- [ ] QR: open the event Share panel → QR dialog renders the blob image → PNG/SVG download attributes differ → copy link succeeds.
- [ ] Plan limit: on Free, creating a second active event shows the 402 message with the `/billing` action.

**Gates**

```text
pnpm lint
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e        # against live Go API + Postgres (MinIO + worker for branding/QR assets)
pnpm gen:api && git diff --exit-code packages/api-client/src/schema.d.ts
go test ./...
```

---

## 8. Definition of Done

- [ ] All master DoD items that apply; no backend changes (if a contract gap is found, OpenAPI lands first).
- [ ] One-time keys never persisted or logged; QR and branding blob URLs revoked on unmount; asset bytes never pass through Next or the Go API.
- [ ] `proxy.ts` protects every dashboard route, including the existing `/events` and `/account` gap.
- [ ] `PLAN_LIMIT_REACHED` in events and uploads links to `/billing`.
- [ ] `pnpm test:e2e` covers devices, branding → gallery, billing, and QR.
- [ ] AGENTS.md updated if new commands or env vars are introduced (expected: none).
- [ ] Reference the F5 doc from the Phase 6/7/8 report follow-ups where they point at missing UI.
- [ ] Completion report written.
