-- +goose Up
ALTER TABLE users ADD COLUMN storage_bytes BIGINT NOT NULL DEFAULT 0;

UPDATE users u
SET storage_bytes = COALESCE((
    SELECT SUM(e.storage_bytes) FROM events e WHERE e.user_id = u.id
), 0);

CREATE TABLE plans (
    id VARCHAR(40) PRIMARY KEY,
    name VARCHAR(80) NOT NULL,
    price_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
    interval VARCHAR(10) NOT NULL DEFAULT 'month',
    limits JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO plans (id, name, price_cents, currency, interval, limits) VALUES
('free', 'Free', 0, 'MYR', 'month',
 '{"activeEvents":1,"photosPerEvent":500,"storageBytes":5368709120,"retentionDays":7,"apiAccess":false}'::jsonb),
('starter', 'Starter', 2900, 'MYR', 'month',
 '{"activeEvents":5,"photosPerEvent":5000,"storageBytes":53687091200,"retentionDays":30,"apiAccess":false}'::jsonb),
('pro', 'Pro', 5900, 'MYR', 'month',
 '{"activeEvents":0,"photosPerEvent":20000,"storageBytes":536870912000,"retentionDays":90,"apiAccess":true}'::jsonb),
('business', 'Business', 9900, 'MYR', 'month',
 '{"activeEvents":0,"photosPerEvent":0,"storageBytes":1099511627776,"retentionDays":365,"apiAccess":true}'::jsonb);

CREATE TABLE subscriptions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    plan_id VARCHAR(40) NOT NULL REFERENCES plans(id),
    status VARCHAR(30) NOT NULL,
    provider VARCHAR(30),
    provider_ref TEXT,
    interval VARCHAR(10) NOT NULL DEFAULT 'month',
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subs_user ON subscriptions(user_id, status);
CREATE UNIQUE INDEX idx_subs_non_terminal_user ON subscriptions(user_id)
    WHERE status IN ('trialing', 'active', 'past_due');
CREATE UNIQUE INDEX idx_subs_provider_ref ON subscriptions(provider_ref)
    WHERE provider_ref IS NOT NULL;

CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
    amount_cents INT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
    status VARCHAR(20) NOT NULL,
    provider_ref TEXT,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    paid_at TIMESTAMPTZ
);
CREATE INDEX idx_invoices_user_issued ON invoices(user_id, issued_at DESC, id DESC);
CREATE UNIQUE INDEX idx_invoices_provider_ref ON invoices(provider_ref)
    WHERE provider_ref IS NOT NULL;

-- Webhook idempotency by provider event id. A dedicated table keeps
-- redelivery state independent of subscription/invoice lifecycle changes.
CREATE TABLE billing_webhook_events (
    provider VARCHAR(30) NOT NULL,
    provider_ref TEXT NOT NULL,
    event_type VARCHAR(60) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, provider_ref)
);

-- +goose Down
DROP TABLE IF EXISTS billing_webhook_events;
DROP TABLE IF EXISTS invoices;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
ALTER TABLE users DROP COLUMN IF EXISTS storage_bytes;
