package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Backoff computes how long to wait before the next attempt. It grows
// exponentially and is capped so a large attempt count cannot overflow.
func Backoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	if attempts > 9 {
		return 5 * time.Minute
	}
	d := time.Duration(1<<uint(attempts-1)) * time.Second
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}

type PostgresQueue struct {
	pool *pgxpool.Pool
}

func NewPostgresQueue(pool *pgxpool.Pool) *PostgresQueue {
	return &PostgresQueue{pool: pool}
}

const jobColumns = `id, type, payload, status, attempts, max_attempts, last_error,
	run_at, locked_at, locked_by, created_at, updated_at`

func scanJob(row pgx.Row) (*Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Attempts, &j.MaxAttempts,
		&j.LastError, &j.RunAt, &j.LockedAt, &j.LockedBy, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoJob
		}
		return nil, err
	}
	return &j, nil
}

func (q *PostgresQueue) Enqueue(ctx context.Context, jobType string, payload any) (uuid.UUID, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = q.pool.QueryRow(ctx,
		`INSERT INTO jobs (type, payload) VALUES ($1, $2) RETURNING id`, jobType, raw).Scan(&id)
	return id, err
}

// EnqueueUnique enqueues a job unless an instance of the same type is already
// pending or running. It is used by the periodic scheduler; concurrent callers
// may still race, so handlers must stay idempotent.
func (q *PostgresQueue) EnqueueUnique(ctx context.Context, jobType string, payload any) (uuid.UUID, bool, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return uuid.Nil, false, err
	}
	var id uuid.UUID
	err = q.pool.QueryRow(ctx,
		`INSERT INTO jobs (type, payload)
		 SELECT $1::text, $2::jsonb
		 WHERE NOT EXISTS (
			 SELECT 1 FROM jobs WHERE type = $1::text AND status IN ($3::text, $4::text)
		 )
		 RETURNING id`, jobType, raw, StatusPending, StatusRunning).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, false, err
	}
	return id, true, nil
}

func (q *PostgresQueue) Claim(ctx context.Context, workerID string) (*Job, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`SELECT `+jobColumns+` FROM jobs
		 WHERE status = $1 AND run_at <= NOW()
		 ORDER BY run_at
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`, StatusPending)
	job, err := scanJob(row)
	if err != nil {
		return nil, err
	}

	row = tx.QueryRow(ctx,
		`UPDATE jobs SET status = $2, locked_at = NOW(), locked_by = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+jobColumns, job.ID, StatusRunning, workerID)
	job, err = scanJob(row)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return job, nil
}

func (q *PostgresQueue) Complete(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, locked_at = NULL, locked_by = NULL, last_error = NULL, updated_at = NOW()
		 WHERE id = $1`, id, StatusDone)
	return err
}

func (q *PostgresQueue) Fail(ctx context.Context, id uuid.UUID, cause string) (bool, error) {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`UPDATE jobs SET attempts = attempts + 1, last_error = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING attempts, max_attempts`, id, cause)
	var attempts, maxAttempts int
	if err := row.Scan(&attempts, &maxAttempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, ErrNoJob
		}
		return false, err
	}

	if attempts >= maxAttempts {
		_, err = tx.Exec(ctx,
			`UPDATE jobs SET status = $2, locked_at = NULL, locked_by = NULL, updated_at = NOW()
			 WHERE id = $1`, id, StatusFailed)
		if err != nil {
			return false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}

	_, err = tx.Exec(ctx,
		`UPDATE jobs SET status = $2, run_at = $3, locked_at = NULL, locked_by = NULL, updated_at = NOW()
		 WHERE id = $1`, id, StatusPending, time.Now().Add(Backoff(attempts)))
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (q *PostgresQueue) Release(ctx context.Context, id uuid.UUID) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE jobs SET status = $2, locked_at = NULL, locked_by = NULL, updated_at = NOW()
		 WHERE id = $1 AND status = $3`, id, StatusPending, StatusRunning)
	return err
}
