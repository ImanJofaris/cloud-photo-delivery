-- +goose Up
ALTER TABLE event_settings ADD COLUMN password_changed_at TIMESTAMPTZ;

CREATE INDEX idx_photos_event_status_created ON photos(event_id, status, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_photos_event_status_created;

ALTER TABLE event_settings DROP COLUMN password_changed_at;
