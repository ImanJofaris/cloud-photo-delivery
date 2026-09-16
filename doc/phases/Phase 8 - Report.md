# Phase 8 — Billing & Subscriptions — Completion Report

**Status:** COMPLETE
**Completed:** 2026-09-13
**Author:** opencode

> **Post-phase note:** entitlements were hardened and renamed in
> [Phase 8.1](Phase 8.1 - Report.md). `activeEvents` is now `events` (all live
> events: upcoming, active, completed) and the plan limits gained `branding`
> and `originalDownloads`. Read Phase 8.1 for the current semantics.

---

## 1. Summary

Phase 8 turns plans into data and makes entitlements real. Operators can subscribe to a plan, upgrade, downgrade, cancel at period end, resume, and read invoices, while events and uploads enforce active-event, per-event photo, and total-storage limits through the shared `limits.PlanLimits` interface — neither domain imports billing. Payments are provider-agnostic behind a `PaymentProvider` interface, with a `manual` provider that generates offline checkout refs and verifies webhook HMAC signatures, so the whole lifecycle is testable locally with no external calls. `users.storage_bytes` now tracks tenant storage in the same transaction as the existing event counters.

---

## 2. Exit criteria — verification

| Criteria | Result | Evidence |
|---|---|---|
| Operator can subscribe | PASS | `POST /billing/subscribe` returns an active subscription + open invoice + checkout session; `TestSubscribe_CreatesActiveSubscriptionInvoiceAndCheckout`; E2E `TestE2E_BillingFlow` |
| Limits are enforced | PASS | Events: `activeEvents` → `PLAN_LIMIT_REACHED` (402) in `TestBillingEntitlements_BlockEventCreationAtBoundary`; uploads: `photosPerEvent` + `storageBytes` in `TestBillingEntitlements_BlockUploadInitAtBoundary`; unit tests in `internal/billing/entitlements_test.go` and `internal/uploads/service_test.go` |
| Upgrade/downgrade work | PASS | `TestUpgrade_ChangesPlanImmediately`, `TestDowngrade_SoftlyChangesPlanEvenOverLimits` (no data deletion), handler tests |
| Entitlements reflected in the app | PASS | `GET /billing/subscription` returns `{subscription, plan, usage}`; plan limits (activeEvents/photosPerEvent/storageBytes/retentionDays/apiAccess) are exposed as JSON data |
| Webhook is signature-verified and idempotent | PASS | `TestManual_ParseWebhookRejectsInvalidSignature`, `TestManual_ParseWebhookRejectsMissingSignature`, `TestHandleWebhook_InvoicePaidAppliesOnce`, `TestBillingRepository_WebhookIdempotencyConstraint` |
| OpenAPI documents billing endpoints + webhook | PASS | `api/openapi.yaml` v0.8.0: 9 billing paths + `Plan`, `Subscription`, `Invoice`, `BillingWebhookPayload` schemas; `pnpm gen:api` regenerated `packages/api-client/src/schema.d.ts` |
| `manual`/stub provider allows full local dev without external calls | PASS | `internal/billing/provider/manual.go`; E2E drives subscribe → signed webhook → invoice paid → cancel → expiry entirely offline |
| Migration reversible + backfills existing storage | PASS | `TestMigrations_BillingUpDownRoundTrip` asserts tables/column drop and that a pre-existing event's 500 bytes backfill to `users.storage_bytes` |

---

## 3. What was built

### 3.1 Shared limits interface (`internal/platform/limits`)

- `PlanLimits` gains `MaxPhotosPerEvent` and `MaxStorageBytes`; `Default` still grants everything and `0` means unlimited everywhere.
- `internal/billing/entitlements.go` implements the interface by resolving the tenant's active subscription → plan limits, falling back to the seeded `free` plan.

### 3.2 Billing domain (`internal/billing`)

| File | Responsibility |
|---|---|
| `model.go` | `Status` (trialing/active/past_due/canceled/expired), `Plan`, `Subscription`, `Invoice`, `PlanLimits`, usage/view types |
| `provider/provider.go` | `PaymentProvider` interface (`Name`, `CreateCheckout`, `Cancel`, `ParseWebhook`) + event/session types |
| `provider/manual.go` | Offline provider: `manual_checkout_*` refs, `manual://checkout/...` URL, HMAC-SHA256 body verification (`X-Billing-Signature`, `sha256=` prefix optional), `Sign` helper for local clients/tests |
| `entitlements.go` | DB-backed `limits.PlanLimits` implementation (free-plan fallback) |
| `service.go` | subscribe/upgrade/downgrade/cancel/resume/invoices/webhook; valid transitions only; lazy expiry when a canceled period ends; soft downgrade (never deletes data) |
| `repository.go` | All SQL/pgx: plans, subscriptions, invoices, webhook idempotency, usage counters |
| `handler.go` | Parse + service call + envelope; actor from `auth.UserID(ctx)` |
| `cursor.go` | Invoice keyset pagination over `(issued_at, id)` |

### 3.3 Wiring (`cmd/api/main.go`)

- `billingRepo` + `provider.NewManual(cfg.BillingWebhookSecret)` + `billing.Service` + `billing.NewEntitlements`.
- `events.NewService(..., entitlements, ...)` replaces `limits.NewDefault()`; `uploads.NewService(..., entitlements)` gains the new dependency.
- Billing routes live inside the `RequireAuth` group; `POST /billing/webhook` is registered outside it (unauthenticated, signature-verified).

### 3.4 Uploads enforcement (`internal/uploads`)

- `Initialize` validates the request, then checks `photosPerEvent` (non-FAILED photo count) and `storageBytes` (`users.storage_bytes + requested size`) before creating the photo row. Limit errors are `PLAN_LIMIT_REACHED` (402) and name the limit in the message.

### 3.5 Storage accounting (`internal/photos`)

- `MarkProcessing` increments `users.storage_bytes` and `DeleteOwned` decrements it in the same transaction as `events.photo_count`/`events.storage_bytes`; `CountByEvent` (excluding FAILED) backs the photo quota.

### 3.6 Config

- `BILLING_PROVIDER` (default `manual`, validated) and `BILLING_WEBHOOK_SECRET` (required, never logged); `.env.example` updated; config unit tests cover defaults and validation failures.

---

## 4. Database changes

```text
migrations/0009_billing.sql
```

- `users.storage_bytes BIGINT NOT NULL DEFAULT 0`, backfilled from `SUM(events.storage_bytes)` per user.
- `plans` (JSONB `limits`) seeded with free/starter/pro/business; free = 1 active event, 500 photos/event, 5 GiB, 7 days; starter = 5 / 5,000 / 50 GiB / 30 days; pro = unlimited events / 20,000 / 500 GiB / 90 days / API; business = unlimited events+photos / 1 TiB / 365 days / API.
- `subscriptions` (adds `interval` for the `{planId, interval}` request), with a partial unique index `idx_subs_non_terminal_user` enforcing one non-terminal subscription per user and `idx_subs_provider_ref` for webhook lookup.
- `invoices` with `(user_id, issued_at DESC, id DESC)` index for cursor pagination.
- `billing_webhook_events` with `PRIMARY KEY (provider, provider_ref)` as the idempotency constraint. Chosen over a column on subscriptions/invoices so redelivery state is independent of lifecycle changes (an invoice may be voided, a subscription plan changed) and so events that don't map to a row are still recorded.
- Down drops all four tables and `users.storage_bytes`.

---

## 5. API surface added

```http
GET  /api/v1/billing/plans
GET  /api/v1/billing/subscription
POST /api/v1/billing/subscribe        # { planId, interval? }
POST /api/v1/billing/upgrade          # { planId }
POST /api/v1/billing/downgrade        # { planId }
POST /api/v1/billing/cancel
POST /api/v1/billing/resume
GET  /api/v1/billing/invoices         # ?cursor=&limit=
POST /api/v1/billing/webhook          # unauthenticated, HMAC-signed
```

Subscribe example:

```http
POST /api/v1/billing/subscribe
Authorization: Bearer <token>
{"planId":"starter"}

201 {"data":{"subscription":{"id":"...","planId":"starter","status":"active",
      "provider":"manual","providerRef":"manual_checkout_...","cancelAtPeriodEnd":false,...},
     "checkout":{"providerRef":"manual_checkout_...","url":"manual://checkout/...",
      "expiresAt":"2026-09-14T12:00:00Z"}},"error":null}
```

Limit failure example:

```http
POST /api/v1/events {"name":"Two","status":"active"}
402 {"data":null,"error":{"code":"PLAN_LIMIT_REACHED",
     "message":"activeEvents limit reached for your plan"}}
```

---

## 6. Files created / modified

```text
api/openapi.yaml                                  (v0.8.0, billing paths + schemas)
migrations/0009_billing.sql                       (new)
internal/platform/limits/limits.go
internal/platform/config/config.go
internal/platform/config/config_test.go
internal/billing/model.go                         (new)
internal/billing/entitlements.go                  (new)
internal/billing/service.go                       (new)
internal/billing/repository.go                    (new)
internal/billing/handler.go                       (new)
internal/billing/cursor.go                        (new)
internal/billing/provider/provider.go             (new)
internal/billing/provider/manual.go               (new)
internal/billing/fakes_test.go                    (new)
internal/billing/service_test.go                  (new)
internal/billing/entitlements_test.go             (new)
internal/billing/handler_test.go                  (new)
internal/billing/provider/manual_test.go          (new)
internal/billing/repository_integration_test.go   (new)
internal/events/service.go                        (limit message names activeEvents)
internal/events/service_test.go
internal/uploads/service.go                       (limits dependency + enforcement)
internal/uploads/repository.go                    (UserStorageBytes)
internal/uploads/fakes_test.go
internal/uploads/service_test.go
internal/uploads/handler_test.go
internal/photos/repository.go                     (CountByEvent, users.storage_bytes)
internal/photos/repository_integration_test.go
internal/photos/service_test.go
internal/photos/processor_test.go
cmd/api/main.go                                   (billing wiring + routes)
test/e2e/billing_flow_test.go                     (new)
test/e2e/uploads_flow_test.go
migrations/migrations_integration_test.go
.env.example
packages/api-client/src/schema.d.ts               (regenerated)
```

---

## 7. Tests

### Unit

- `entitlements_test.go`: free-plan fallback, active plan override, unlimited (`0`) bypass, invalid user id.
- `service_test.go`: subscribe (month/year amounts, periods), unknown/inactive plan, duplicate active subscription, upgrade/downgrade direction rules, soft downgrade while over limits, cancel/resume transitions, lazy expiry to free plan, invoice cursor + tenant isolation, webhook apply/duplicate/unknown/missing-field/compensation paths.
- `handler_test.go`: plans, subscription view, subscribe/upgrade/downgrade/cancel/resume envelopes, 404/409/422 mapping, webhook 401/400/202, invalid cursor, unauthorized.
- `provider/manual_test.go`: offline checkout, cancel no-op, valid/bare/absent/invalid signatures, malformed and missing fields, empty secret.
- `internal/uploads/service_test.go`: `photosPerEvent` and `storageBytes` blocks, unlimited bypass, limit-lookup internal error.

### Integration (`//go:build integration`)

- `internal/billing/repository_integration_test.go`: plan parse + subscription lifecycle, unique non-terminal subscription per user, webhook idempotency constraint, invoice cursor pagination, `ErrNotFound` paths, usage counters; boundary tests proving the real entitlements block event creation and upload init; tenant isolation.
- `migrations/migrations_integration_test.go`: billing up/down round trip, seed count, storage backfill.
- `internal/photos/repository_integration_test.go`: `users.storage_bytes` increments/decrements in the same transaction and `CountByEvent` excludes FAILED.

### E2E

- `test/e2e/billing_flow_test.go`: free user creates one active event → second active event 402 `PLAN_LIMIT_REACHED` → subscribe Starter → signed `invoice.paid` webhook → invoice paid → second active event succeeds → cancel at period end (status stays `active`) → period end passes → status `expired`, plan falls back to free, active-event limit applies again.

### Verification commands and results

```text
gofmt -l .                                                    -> (nothing)
go build ./...                                                -> exit 0
go vet ./...                                                  -> exit 0
go vet -tags=integration ./...                                -> exit 0
go test ./...                                                 -> all packages ok
go test -tags=integration -p 1 -timeout 40m -count=1 ./...    -> all packages ok (e2e 53.3s)
go test -tags=integration -p 1 -cover ./internal/billing/     -> 84.6% (floor 80%)
go test -cover ./internal/billing/provider/                   -> 90.6%
pnpm install / gen:api / lint / typecheck / test              -> all pass (79 web tests, 7 client tests)
```

`-p 1` is required on this Windows machine: parallel package tests race testcontainers' provider detection (`rootless Docker is not supported on Windows`); serial runs are stable. CI on Linux runs the plain command.

---

## 8. Issues found and fixed

| Issue | Fix |
|---|---|
| Parallel integration packages intermittently fail with `rootless Docker is not supported on Windows` | Re-ran with `-p 1` (same workaround documented in Phases 3–6 reports); no code change |
| Webhook effects could be lost if the process died between recording the event and applying it | `HandleWebhook` deletes the idempotency row when applying fails so providers can retry; covered by `TestHandleWebhook_CompensatesWhenApplyFails` |
| Existing tenants would have under-counted storage after adding `users.storage_bytes` | Migration backfills from `SUM(events.storage_bytes)`; asserted in the migration round-trip test |
| Config would start without a webhook secret, silently making signature verification impossible | `BILLING_WEBHOOK_SECRET` is required and `BILLING_PROVIDER` validated at startup |

---

## 9. Known limitations / follow-ups

- `retentionDays` is exposed in plan limits but not enforced here; the Phase 9 `event.expire` scheduler owns retention-based expiry (per the phase plan).
- `apiAccess` is surfaced in the plan payload but not enforced by a new gate; Phase 6 device/API-key access predates this flag.
- Renewal is not simulated: only `checkout.completed`/`invoice.paid`/`subscription.canceled` are handled, and `invoice.paid` does not extend `current_period_end`. A real gateway adapter should extend the period on renewal webhooks.
- Lazy expiry only runs on `GET /billing/subscription`; a Phase 9 scheduler should expire canceled subscriptions proactively.
- Only the `manual` provider is implemented; Billplz/Stripe adapters slot into `PaymentProvider` later.
- The operator billing UI (plans, subscription lifecycle, invoices, usage meters) ships in Phase F5 (`doc/phases/Phase F5 - Devices, Branding and Billing.md`).

---

## 10. How to try it

```text
# 1. Run infrastructure + API (set env vars; .env is not auto-loaded)
docker compose up -d
$env:HTTP_ADDR=":18080"; $env:DATABASE_URL="postgres://cpd:cpd@localhost:5432/cpd?sslmode=disable"
$env:JWT_SECRET="dev-only-change-me"; $env:BILLING_WEBHOOK_SECRET="dev-only-change-me"
goose -dir migrations postgres "$env:DATABASE_URL" up
go run ./cmd/api

# 2. Sign up, create an active event, hit the free limit
#    (curl.exe; pass JSON via temp files on PowerShell)

# 3. Subscribe
POST /api/v1/billing/subscribe {"planId":"starter"}

# 4. Sign and send a webhook (PowerShell HMAC)
$body = '{"id":"evt-1","type":"invoice.paid","providerRef":"<providerRef>"}'
$mac = [System.Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes("dev-only-change-me"))
$sig = ($mac.ComputeHash([Text.Encoding]::UTF8.GetBytes($body)) | ForEach-Object { $_.ToString("x2") }) -join ""
# POST /api/v1/billing/webhook with header X-Billing-Signature: $sig

# 5. Verify
GET /api/v1/billing/subscription   # plan starter, usage
GET /api/v1/billing/invoices       # one paid invoice
POST /api/v1/billing/cancel        # cancelAtPeriodEnd: true
```
