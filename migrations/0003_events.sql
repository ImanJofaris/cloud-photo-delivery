-- +goose Up
CREATE TABLE events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) NOT NULL,
    client_name VARCHAR(255),
    client_email VARCHAR(320),
    location VARCHAR(255),
    description TEXT,
    event_date DATE,
    status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
    cover_photo_id UUID,
    storage_bytes BIGINT NOT NULL DEFAULT 0,
    photo_count BIGINT NOT NULL DEFAULT 0,
    guest_count BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT events_status_check CHECK (status IN ('upcoming', 'active', 'completed', 'archived')),
    CONSTRAINT events_slug_unique UNIQUE (user_id, slug)
);

CREATE TABLE event_settings (
    event_id UUID PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    visibility VARCHAR(30) NOT NULL DEFAULT 'public',
    password_hash TEXT,
    allow_download BOOLEAN NOT NULL DEFAULT TRUE,
    allow_original_download BOOLEAN NOT NULL DEFAULT FALSE,
    watermark_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT event_settings_visibility_check CHECK (visibility IN ('public', 'password', 'private'))
);

CREATE INDEX idx_events_user_status ON events(user_id, status);
CREATE INDEX idx_events_user_created ON events(user_id, created_at DESC, id DESC);
CREATE INDEX idx_events_user_deleted ON events(user_id, deleted_at);

-- +goose Down
DROP TABLE IF EXISTS event_settings;
DROP TABLE IF EXISTS events;
