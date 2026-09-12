-- +goose Up
ALTER TABLE jobs ADD COLUMN max_attempts INT NOT NULL DEFAULT 5;
ALTER TABLE jobs ADD COLUMN locked_at TIMESTAMPTZ;
ALTER TABLE jobs ADD COLUMN locked_by TEXT;

DROP INDEX IF EXISTS idx_jobs_pending;
CREATE INDEX idx_jobs_ready ON jobs(status, run_at) WHERE status = 'pending';
CREATE INDEX idx_jobs_type ON jobs(type, status);

-- +goose Down
DROP INDEX IF EXISTS idx_jobs_type;
DROP INDEX IF EXISTS idx_jobs_ready;
CREATE INDEX idx_jobs_pending ON jobs(status, run_at);

ALTER TABLE jobs DROP COLUMN locked_by;
ALTER TABLE jobs DROP COLUMN locked_at;
ALTER TABLE jobs DROP COLUMN max_attempts;
