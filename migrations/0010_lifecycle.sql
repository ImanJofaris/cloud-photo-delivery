-- +goose Up
ALTER TABLE events DROP CONSTRAINT events_status_check;
ALTER TABLE events ADD CONSTRAINT events_status_check
    CHECK (status IN ('upcoming', 'active', 'completed', 'archived', 'expired'));

ALTER TABLE events ADD COLUMN expiry_warned_at TIMESTAMPTZ;

CREATE INDEX idx_events_expires_at ON events(expires_at)
    WHERE deleted_at IS NULL AND expires_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_events_expires_at;
ALTER TABLE events DROP COLUMN IF EXISTS expiry_warned_at;

ALTER TABLE events DROP CONSTRAINT events_status_check;
UPDATE events SET status = 'archived' WHERE status = 'expired';
ALTER TABLE events ADD CONSTRAINT events_status_check
    CHECK (status IN ('upcoming', 'active', 'completed', 'archived'));
