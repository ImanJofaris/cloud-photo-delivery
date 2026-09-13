-- +goose Up
CREATE TABLE devices (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    key_hash TEXT NOT NULL,
    assigned_event_id UUID REFERENCES events(id) ON DELETE SET NULL,
    revoked_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_user ON devices(user_id);
CREATE UNIQUE INDEX idx_devices_prefix ON devices(key_prefix);

-- +goose Down
DROP INDEX IF EXISTS idx_devices_prefix;
DROP INDEX IF EXISTS idx_devices_user;
DROP TABLE devices;
