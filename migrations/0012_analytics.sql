-- +goose Up
CREATE TABLE event_analytics (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    gallery_views BIGINT NOT NULL DEFAULT 0,
    unique_visitors BIGINT NOT NULL DEFAULT 0,
    downloads BIGINT NOT NULL DEFAULT 0,
    qr_scans BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (event_id, day)
);

CREATE TABLE event_visitors (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    day DATE NOT NULL,
    visitor_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, day, visitor_hash)
);

-- +goose Down
DROP TABLE IF EXISTS event_visitors;
DROP TABLE IF EXISTS event_analytics;
