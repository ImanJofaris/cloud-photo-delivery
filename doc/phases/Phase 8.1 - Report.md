# Phase 8.1 — Entitlement Hardening — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-16
**Author:** opencode

---

## 1. Summary

Phase 8.1 closes every plan-abuse gap found after the operator surfaces shipped. The event entitlement now counts every event guests can still reach (`upcoming`, `active`, `completed`) instead of only `active` ones, so a Free tenant cannot run unlimited live galleries; the entitlement and usage fields are renamed from `activeEvents` to `events` to match. Event expiry is clamped to the plan's retention window on create, update, and extend, closing a bypass where any tier could set `expiresAt` years out. The lifecycle is now honest: archived events are hidden from the public gallery and no new uploads are accepted for archived or expired events. Paid features that were previously free are gated server-side — `apiAccess` (Pro+) for devices, `branding` (Starter+) for account branding, and `originalDownloads` (Starter+) for guest-facing original downloads — with upgrade paths in the UI. The billing usage meter counts live events, so customers see their real usage.

---

## 2. Exit criteria — verification

The user requirement: no tier (Free, Starter, Pro, Business) can exceed its plan.

| Criteria | Result | Evidence |
|---|---|---|
| Event limit counts every reachable event | PASS | `TestCreate_LiveEventLimitReached` (upcoming occupies a slot, archive frees it); live smoke: free user second event → `402 PLAN_LIMIT_REACHED "events limit reached for your plan"` |
| Retention cannot be bypassed | PASS | `TestCreate_ExplicitExpiryBeyondRetentionRejected`, `TestUpdate_ExpiryBeyondRetentionRejected`, `TestExtend_BeyondRetentionRejected`, `TestUpdate_ClearExpiryResetsToRetention`; live smoke: free `expiresAt +30d` → `402 "retentionDays limit reached for your plan"`, `+3d` → `201` |
| Archived events stop serving guests | PASS | Gallery repo filters `status <> 'archived'`; `TestService_GetEvent_ArchivedHidden`; live smoke: archived gallery → `404` |
| Archived/expired events stop accepting uploads | PASS | `TestInitialize_RejectsClosedEvents`; live smoke: archived upload init → `409 INVALID_STATUS_TRANSITION "Event is archived; uploads are closed"` |
| Devices require `apiAccess` | PASS | `TestCreate_RequiresAPIAccess`; live smoke: Free/Starter device create → `402`, Pro → `201` |
| Branding requires the branded plan | PASS | `TestBrandingService_Update_RequiresPlan`, `TestBrandingService_CreateAssetUpload_RequiresPlan`; live smoke: Free → `402`, Starter → `200` |
| Original downloads require the plan | PASS | `TestCreate_OriginalDownloadsGated`, `TestUpdateSettings_OriginalDownloadsGated`, `TestService_PhotoURL_PlanDisablesOriginal`; live smoke: Free → `402`, Starter → `200` |
| Usage meter reflects live events | PASS | `CountLiveEvents`; `GET /billing/subscription` returns `usage.events`; web dashboard/usage meters renamed |
| All gates green | PASS | See §7 |

---

## 3. What was built

### 3.1 Entitlements (`internal/platform/limits`, `internal/billing`)

- `PlanLimits` interface: `MaxActiveEvents` → `MaxEvents`; added `APIAccess`, `BrandingEnabled`, `OriginalDownloads`. `limits.Default` grants everything (dev/tests).
- `billing.PlanLimits`: `activeEvents` → `events`; added `branding` and `originalDownloads` booleans. `Usage`: `activeEvents` → `events`.
- `CountLiveEvents` counts `upcoming|active|completed` (`deleted_at IS NULL`), replacing `CountActiveEvents`.
- `billing.Entitlements` implements the new methods; the plan JSON is the single source of truth.

### 3.2 Events (`internal/events`)

- `enforceEventLimit` runs on every create (not only `active`) and on `expired → active` transitions; error names `events`.
- `checkRetention` rejects `expiresAt` later than `now + retentionDays` on create, `PATCH`, and `extend` (`PLAN_LIMIT_REACHED`). Clearing the expiry on a plan with retention resets it to `now + retentionDays` instead of never.
- `resolveExpiry` derives from retention when omitted; explicit values are validated against it.
- Settings writes (`create` and `PATCH /events/{id}/settings`) reject `allowOriginalDownload: true` when the plan lacks `originalDownloads`.

### 3.3 Uploads (`internal/uploads`)

- New `Repository.EventState` (tenant-scoped) returns event status + expiry.
- `enforceEventOpen` runs in `Initialize` only: new uploads are rejected `409 INVALID_STATUS_TRANSITION` for `archived`/`expired` (or past `expires_at`); finishing already-initialized uploads is unaffected.

### 3.4 Gallery (`internal/gallery`)

- `EventBySlug` hides `archived` events in addition to deleted/expired ones.
- New `DownloadPolicy` dependency (`OriginalDownloads`): `effectiveSettings` degrades `allowOriginalDownload` to `false` whenever the plan lacks it, so both the public event DTO and `PhotoURL(original)` respect the entitlement even for grandfathered events.

### 3.5 Devices & branding (`internal/devices`, `internal/users`)

- `devices.Service` takes a narrow `PlanLimits` (`APIAccess`) and rejects `Create` with `402 apiAccess`.
- `BrandingService` takes a narrow `PlanLimits` (`BrandingEnabled`) and rejects `Update` and `CreateAssetUpload` with `402 branding`.

### 3.6 Web (`apps/web`)

- Billing: usage meter label "Events" (`usage.events` / `limits.events`), plan card "N events at a time", dashboard summary renamed.
- Event status card copy: "Events you can still share count toward your plan's limit. Archive one to free a slot."
- Devices dialog: `PLAN_LIMIT_REACHED` shows `View plans → /billing`.
- Branding page: plans without branding render an upgrade card ("Custom branding is a Starter feature") instead of the form.
- Plan-limit E2E now drives the real create-blocked path (no `page.route`); devices/branding/lifecycle/analytics E2E subscribe via the API before using gated features.

---

## 4. Database changes

```text
migrations/0014_entitlements.sql
```

- Up: rewrites `plans.limits` — moves `activeEvents` → `events`, adds `branding` and `originalDownloads` (`id <> 'free'`).
- Down: removes the new keys and restores `activeEvents`.
- Reversible; covered by `TestMigrations_*` up/down round trip (`-count=1`, 60s).

Note: only `plans.limits` JSON changes; no table/column changes.

---

## 5. API surface added / changed

No new endpoints; contract changes:

```http
PATCH  /api/v1/events/{eventID}            # 402 when expiresAt exceeds retention; empty string resets to retention
PATCH  /api/v1/events/{eventID}/settings   # 402 when allowOriginalDownload is not in the plan
POST   /api/v1/events/{eventID}/extend     # 402 when the result exceeds retention
POST   /api/v1/events                      # 402 now covers every status (live-event limit) + retention
POST   /api/v1/events/{eventID}/uploads    # 409 when the event is archived/expired
POST   /api/v1/devices                     # 402 when the plan lacks apiAccess
PATCH  /api/v1/account/branding            # 402 when the plan lacks branding
POST   /api/v1/account/branding/assets     # 402 when the plan lacks branding
GET    /api/v1/public/events/{slug}        # archived events now 404
```

Schemas: `BillingPlanLimits` (`events`, `branding`, `originalDownloads`), `SubscriptionUsage` (`events`); effective `allowOriginalDownload` documented on `PublicEvent`.

Example:

```json
{ "data": null, "error": { "code": "PLAN_LIMIT_REACHED", "message": "events limit reached for your plan" } }
```

---

## 6. Files created / modified

```text
internal/platform/limits/limits.go
internal/billing/{model,entitlements,repository,service,handler}.go
internal/billing/{fakes,entitlements,service,handler,repository_integration}_test.go
internal/events/{service,repository}.go
internal/events/{service,handler}_test.go
internal/uploads/{service,repository}.go
internal/uploads/{service,fakes}_test.go
internal/gallery/{service,repository}.go
internal/gallery/{service,fakes}_test.go
internal/gallery/repository_integration_test.go
internal/devices/{service,service_test}.go
internal/users/{branding,branding_test}.go
cmd/api/main.go
api/openapi.yaml
packages/api-client/src/schema.d.ts            # regenerated
apps/web/src/features/billing/{usage-meters,plan-card}.tsx
apps/web/src/app/(dashboard)/dashboard/page.tsx
apps/web/src/features/events/{errors.ts,status-actions.tsx,status-actions.test.tsx}
apps/web/src/features/devices/{errors.ts,device-form-dialog.tsx}
apps/web/src/features/branding/{branding-form.tsx,branding-form.test.tsx}
apps/web/e2e/{plan-limit,devices,branding,lifecycle,analytics,billing}.spec.ts
migrations/0014_entitlements.sql
```

---

## 7. Tests

### Unit
- Events: live-event limit (upcoming counts, archive frees), retention on create/PATCH/extend, clear-expiry reset, original-download gating on create/settings, reactivation limit.
- Uploads: archived/expired/past-expiry initialize rejected; completion unaffected.
- Gallery: archived slug 404; plan without originals disables the original URL and DTO flag.
- Devices: `apiAccess` gate with no row written.
- Branding: both write paths gated with no side effects.
- Billing: entitlements for the renamed/new limits; usage uses live count.

### Integration (`//go:build integration`)
- `internal/billing`: free plan blocks the second live event and Starter lifts it; `CountLiveEvents`.
- `internal/events`, `internal/gallery`: repositories against real Postgres.
- `migrations`: full up/down round trip including 0014.

### E2E
- Go: `test/e2e` event/billing flows updated for the renamed entitlement.
- Playwright: full suite **17 passed (1.7m)**, including the new gates and the free-plan create-blocked flow.

### Verification commands and results

```text
gofmt -l .                       -> clean
go build ./...                   -> ok
go vet ./...                     -> clean
go vet -tags=integration ./...   -> clean
go test ./...                    -> all packages ok
go test -tags=integration -p 1 ./migrations ./internal/billing ./internal/gallery ./internal/events -> ok
pnpm lint (web)                  -> clean
pnpm typecheck                   -> clean
pnpm test                        -> 243 passed
pnpm build                       -> ok
pnpm test:e2e                    -> 17 passed (1.7m)
```

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Explicit `expiresAt` on create/PATCH and `Extend` ignored `retentionDays`, giving every tier unlimited retention | `checkRetention` on all three paths; clearing the expiry resets to the plan window |
| Event limit counted only `active`, but galleries serve `upcoming`/`completed`, so tiers could run unlimited live events | Live-event counting + enforcement on every create; archive/expiry frees a slot |
| Danger zone says "Archiving hides the gallery" but `EventBySlug` did not filter archived | Added `status <> 'archived'` |
| Uploads were accepted for archived/expired events | `enforceEventOpen` on initialize (409) |
| `apiAccess`, branding, and original downloads were exposed and configurable on every plan | Narrow `PlanLimits` gates in devices/branding/events/gallery + UI upgrade paths |
| The usage meter understated usage (active-only) | `CountLiveEvents`; meter/dashboard renamed to "Events" |

---

## 9. Known limitations / follow-ups

- **Derivative downloads remain free on all plans.** Free guests can still download the optimized `medium`/`large` variants and the gallery is viewable; only `original` is gated (per decision). Revisit if the Free tier should be view-only.
- **Live-event counting is status-based, not expiry-based.** An event past `expires_at` keeps its slot until the 15-minute expire job flips it to `expired`. Bounded and acceptable; align if needed.
- **Branding downgrade keeps stored assets.** A tenant that downgrades keeps its branding row; the gallery still shows it (read path is not gated). Gating the read path would need a product call.
- **Devices created before this change keep working** after a downgrade; only new device creation is gated.
- **Per-instance rate limits** unchanged (Phase 10 limitation).

---

## 10. How to try it

```powershell
docker compose up -d
goose -dir migrations postgres "$env:DATABASE_URL" up
$env:HTTP_ADDR = ":18080"; $env:DATABASE_URL = "postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable"
$env:JWT_SECRET = "dev-only-change-me"; $env:BILLING_WEBHOOK_SECRET = "dev-webhook-secret"
go run ./cmd/api

# Free: first event ok, second blocked
curl.exe -s -X POST http://localhost:18080/api/v1/events -H "Authorization: Bearer <token>" `
  -H "Content-Type: application/json" --data-binary '{"name":"Second"}'
# -> 402 PLAN_LIMIT_REACHED "events limit reached for your plan"

# Free: expiry beyond 7 days blocked
... --data-binary '{"name":"Far","expiresAt":"2027-01-01T00:00:00Z"}'
# -> 402 PLAN_LIMIT_REACHED "retentionDays limit reached for your plan"

# Free: branding / devices / original downloads blocked; subscribe Starter/Pro to unlock
```
