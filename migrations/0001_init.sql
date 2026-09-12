-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS schema_migrations_baseline (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS schema_migrations_baseline;
