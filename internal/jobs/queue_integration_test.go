//go:build integration

package jobs_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupQueue(t *testing.T) (*pgxpool.Pool, *jobs.PostgresQueue) {
	t.Helper()
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cpd"),
		postgres.WithUsername("cpd"),
		postgres.WithPassword("cpd"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	_, err = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type VARCHAR(50) NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}'::jsonb,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		attempts INT NOT NULL DEFAULT 0,
		max_attempts INT NOT NULL DEFAULT 5,
		last_error TEXT,
		locked_at TIMESTAMPTZ,
		locked_by TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	require.NoError(t, err)
	return pool, jobs.NewPostgresQueue(pool)
}

func TestQueue_ClaimMarksRunningAndSetsLock(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, "test.job", map[string]string{"k": "v"})
	require.NoError(t, err)

	job, err := q.Claim(ctx, "worker-1")
	require.NoError(t, err)
	require.Equal(t, id, job.ID)
	require.Equal(t, jobs.StatusRunning, job.Status)
	require.NotNil(t, job.LockedBy)
	require.Equal(t, "worker-1", *job.LockedBy)
	require.NotNil(t, job.LockedAt)

	var payload map[string]string
	require.NoError(t, json.Unmarshal(job.Payload, &payload))
	require.Equal(t, "v", payload["k"])

	var status string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM jobs WHERE id = $1`, id).Scan(&status))
	require.Equal(t, "running", status)
}

func TestQueue_ClaimReturnsNoJobWhenEmpty(t *testing.T) {
	_, q := setupQueue(t)
	_, err := q.Claim(context.Background(), "worker-1")
	require.ErrorIs(t, err, jobs.ErrNoJob)
}

func TestQueue_OrderingByRunAt(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	first, _ := q.Enqueue(ctx, "test.job", nil)
	second, _ := q.Enqueue(ctx, "test.job", nil)

	_, err := pool.Exec(ctx, `UPDATE jobs SET run_at = NOW() + INTERVAL '1 hour' WHERE id = $1`, second)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE jobs SET run_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, first)
	require.NoError(t, err)

	job, err := q.Claim(ctx, "w")
	require.NoError(t, err)
	require.Equal(t, first, job.ID)
}

func TestQueue_FutureJobNotClaimed(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, "test.job", nil)
	_, err := pool.Exec(ctx, `UPDATE jobs SET run_at = NOW() + INTERVAL '1 hour' WHERE id = $1`, id)
	require.NoError(t, err)

	_, err = q.Claim(ctx, "w")
	require.ErrorIs(t, err, jobs.ErrNoJob)
}

func TestQueue_ConcurrentClaimNeverReturnsSameJob(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	const n = 20
	ids := make(map[uuid.UUID]bool)
	for i := 0; i < n; i++ {
		id, err := q.Enqueue(ctx, "test.job", nil)
		require.NoError(t, err)
		ids[id] = true
	}

	var mu sync.Mutex
	seen := map[uuid.UUID]int{}
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for {
				job, err := q.Claim(ctx, "w")
				if err != nil {
					return
				}
				mu.Lock()
				seen[job.ID]++
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()

	require.Len(t, seen, n, "every job should be claimed exactly once")
	for id, count := range seen {
		require.Equal(t, 1, count, "job %s claimed %d times", id, count)
	}
	_ = pool
}

func TestQueue_Complete(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, "test.job", nil)
	_, err := q.Claim(ctx, "w")
	require.NoError(t, err)
	require.NoError(t, q.Complete(ctx, id))

	var status string
	var locked *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, locked_by FROM jobs WHERE id = $1`, id).Scan(&status, &locked))
	require.Equal(t, "done", status)
	require.Nil(t, locked)
}

func TestQueue_FailRetriesWithBackoffThenExhausts(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, "test.job", nil)
	_, err := pool.Exec(ctx, `UPDATE jobs SET max_attempts = 3 WHERE id = $1`, id)
	require.NoError(t, err)

	for attempt := 1; attempt <= 2; attempt++ {
		_, err := q.Claim(ctx, "w")
		require.NoError(t, err)
		retry, err := q.Fail(ctx, id, "transient")
		require.NoError(t, err)
		require.True(t, retry, "attempt %d should schedule a retry", attempt)

		var status string
		var attempts int
		var runAt time.Time
		require.NoError(t, pool.QueryRow(ctx,
			`SELECT status, attempts, run_at FROM jobs WHERE id = $1`, id).Scan(&status, &attempts, &runAt))
		require.Equal(t, "pending", status)
		require.Equal(t, attempt, attempts)
		if attempt == 1 {
			require.WithinDuration(t, time.Now().Add(1*time.Second), runAt, 2*time.Second)
		}
		require.WithinDuration(t, time.Now().Add(jobs.Backoff(attempt)), runAt, 2*time.Second)

		_, err = pool.Exec(ctx, `UPDATE jobs SET run_at = NOW() WHERE id = $1`, id)
		require.NoError(t, err)
	}

	_, err = q.Claim(ctx, "w")
	require.NoError(t, err)
	retry, err := q.Fail(ctx, id, "permanent")
	require.NoError(t, err)
	require.False(t, retry, "third attempt should exhaust")

	var status string
	var lastErr string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, last_error FROM jobs WHERE id = $1`, id).Scan(&status, &lastErr))
	require.Equal(t, "failed", status)
	require.Equal(t, "permanent", lastErr)
}

func TestQueue_ReleaseReturnsToPending(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	id, _ := q.Enqueue(ctx, "test.job", nil)
	_, err := q.Claim(ctx, "w")
	require.NoError(t, err)
	require.NoError(t, q.Release(ctx, id))

	var status string
	var attempts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT status, attempts FROM jobs WHERE id = $1`, id).Scan(&status, &attempts))
	require.Equal(t, "pending", status)
	require.Zero(t, attempts, "release must not count an attempt")
}
