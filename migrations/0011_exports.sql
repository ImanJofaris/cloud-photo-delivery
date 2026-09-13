-- +goose Up
CREATE TABLE exports (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL,
    object_key TEXT,
    file_size BIGINT,
    error_message TEXT,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT exports_status_check CHECK (status IN ('pending','processing','ready','failed','expired'))
);

CREATE INDEX idx_exports_event_created ON exports(event_id, created_at DESC, id DESC);
CREATE INDEX idx_exports_status_expires ON exports(status, expires_at);
CREATE INDEX idx_exports_active_event ON exports(event_id)
    WHERE status IN ('pending','processing');

-- +goose Down
DROP TABLE IF EXISTS exports;
