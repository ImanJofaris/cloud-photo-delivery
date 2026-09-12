-- +goose Up
CREATE TABLE photos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    thumbnail_key TEXT,
    optimized_key TEXT,
    medium_key TEXT,
    original_filename TEXT,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    width INT,
    height INT,
    status VARCHAR(30) NOT NULL DEFAULT 'UPLOADING',
    upload_kind VARCHAR(20) NOT NULL DEFAULT 'simple',
    multipart_upload_id TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT photos_status_check CHECK (status IN ('UPLOADING', 'PROCESSING', 'READY', 'FAILED')),
    CONSTRAINT photos_upload_kind_check CHECK (upload_kind IN ('simple', 'multipart'))
);
CREATE INDEX idx_photos_event_created ON photos(event_id, created_at DESC);
CREATE INDEX idx_photos_event_status ON photos(event_id, status);

CREATE TABLE upload_idempotency (
    idempotency_key TEXT PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE upload_parts (
    photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    part_number INT NOT NULL,
    etag TEXT,
    PRIMARY KEY (photo_id, part_number)
);

CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT jobs_status_check CHECK (status IN ('pending', 'running', 'done', 'failed'))
);
CREATE INDEX idx_jobs_pending ON jobs(status, run_at);

-- +goose Down
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS upload_parts;
DROP TABLE IF EXISTS upload_idempotency;
DROP TABLE IF EXISTS photos;
