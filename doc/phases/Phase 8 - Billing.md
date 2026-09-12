# Phase 8 — Billing & Subscriptions

**Goal:** Plans, subscription lifecycle, and storage/feature limits. Payments integrate through a provider-agnostic interface so a Malaysian gateway (e.g. Billplz) or Stripe can be plugged in.

**Depends on:** Phase 2 (events). Limit hooks used by Phases 2/3 become real here.

**Exit criteria:** Operator can subscribe, limits are enforced, upgrade/downgrade work, and entitlements are reflected in the app.

---

## 1. Plans (initial)

```text
Free      RM0   1 active event, 500 photos, 7-day retention, basic gallery, QR
Starter   RM29  5 active events, 5,000 photos/event, 30-day retention, branding, downloads
Pro       RM59   unlimited events, higher storage, 90-day retention, branding, analytics, API
Business  RM99+ large storage, long retention, white label, custom domain, priority support
```

Plans are data, not code: `plans` table + `plan_limits` JSON so pricing can change without deploy.

---

## 2. Database (migration `0009_billing.sql`)

```sql
CREATE TABLE plans (
    id VARCHAR(40) PRIMARY KEY,          -- free|starter|pro|business
    name VARCHAR(80) NOT NULL,
    price_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
    interval VARCHAR(10) NOT NULL DEFAULT 'month',
    limits JSONB NOT NULL,               -- activeEvents, photosPerEvent, storageBytes, retentionDays, apiAccess...
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id VARCHAR(40) NOT NULL REFERENCES plans(id),
    status VARCHAR(30) NOT NULL,         -- trialing|active|past_due|canceled|expired
    provider VARCHAR(30),                -- billplz|stripe|manual
    provider_ref TEXT,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subs_user ON subscriptions(user_id, status);

CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    subscription_id UUID REFERENCES subscriptions(id),
    amount_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
    status VARCHAR(20) NOT NULL,         -- open|paid|void
    provider_ref TEXT,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);
```

Also add `users.storage_bytes BIGINT NOT NULL DEFAULT 0` for plan storage accounting.

---

## 3. API Surface

```http
GET  /api/v1/billing/plans
GET  /api/v1/billing/subscription
POST /api/v1/billing/subscribe              # { planId, interval }
POST /api/v1/billing/upgrade
POST /api/v1/billing/downgrade
POST /api/v1/billing/cancel
POST /api/v1/billing/resume
GET  /api/v1/billing/invoices
POST /api/v1/billing/webhook                # provider callback (unauthenticated, signed)
```

---

## 4. Domain Layout

```text
internal/billing/
  service.go           # subscribe, change plan, entitlements
  entitlements.go      # LimitChecker used by events/uploads
  repository.go
  handler.go
  provider/
    provider.go        # PaymentProvider interface
    billplz.go         # adapter (or stub initially)
    stripe.go          # optional
internal/platform/limits/  # shared interface so events/uploads don't import billing directly
```

`PaymentProvider` interface:

```go
type PaymentProvider interface {
    CreateCheckout(ctx, sub Subscription) (CheckoutSession, error)
    Cancel(ctx, providerRef string) error
    ParseWebhook(r *http.Request) (WebhookEvent, error)
}
```

---

## 5. Rules

- Entitlements check: active event count, photos per event, total storage, retention days, API access (Phase 6).
- Limits surfaced as `PLAN_LIMIT_REACHED` with the limit name; used by events and uploads services via the shared `limits` interface.
- Webhook is signature-verified; idempotent by provider event id.
- Downgrade doesn't delete data; it blocks new usage until within limits (soft enforcement) and updates retention at next expiry run.
- Never store card data; provider handles PCI.

---

## 6. Test Plan

**Unit**
- [ ] entitlements: each plan enforces each limit; unlimited plan bypasses.
- [ ] subscribe/upgrade/downgrade/cancel state transitions.
- [ ] webhook: valid signature applied; invalid rejected; duplicate event id ignored.
- [ ] invoices: generated on payment; status transitions.
- [ ] handler envelope/status.

**Integration**
- [ ] subscription repository + unique active subscription per user.
- [ ] limits actually block event creation (Phase 2 integration) and upload (Phase 3 integration) at boundary.
- [ ] webhook idempotency table/constraint.

**E2E**
- [ ] Free user hits active-event limit → blocked → subscribe Starter → create succeeds.
- [ ] Cancel at period end → status remains active until period end → then expired.

---

## 7. Definition of Done

- [ ] All master DoD items.
- [ ] OpenAPI documents billing endpoints + webhook.
- [ ] A `manual`/stub provider allows full local dev/testing without external calls.
