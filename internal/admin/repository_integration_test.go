//go:build integration

package admin_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/admin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupAdminDB(t *testing.T) *pgxpool.Pool {
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

	mustExecAdmin(t, pool, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	mustExecAdmin(t, pool, `CREATE TABLE users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(320) NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		business_name VARCHAR(255),
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExecAdmin(t, pool, `CREATE TABLE events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL,
		photo_count BIGINT NOT NULL DEFAULT 0,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExecAdmin(t, pool, `CREATE TABLE photos (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		storage_key TEXT NOT NULL,
		file_size BIGINT NOT NULL DEFAULT 0,
		status VARCHAR(20) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExecAdmin(t, pool, `CREATE TABLE subscriptions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		plan_id VARCHAR(50) NOT NULL,
		status VARCHAR(20) NOT NULL,
		interval VARCHAR(20) NOT NULL DEFAULT 'month',
		current_period_end TIMESTAMPTZ,
		cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExecAdmin(t, pool, `CREATE TABLE invoices (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		amount_cents INTEGER NOT NULL,
		currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
		status VARCHAR(20) NOT NULL,
		issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExecAdmin(t, pool, `CREATE TABLE jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type VARCHAR(50) NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}'::jsonb,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	return pool
}

func mustExecAdmin(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func insertAdminUser(t *testing.T, pool *pgxpool.Pool, email string, storage int64, isAdmin bool, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash, storage_bytes, is_admin, created_at)
		 VALUES ($1, 'hash', $2, $3, $4) RETURNING id`,
		email, storage, isAdmin, createdAt).Scan(&id))
	return id
}

func insertAdminEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, slug string, deleted bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	deletedAt := any(nil)
	if deleted {
		deletedAt = time.Now()
	}
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO events (user_id, name, slug, deleted_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, name, slug, deletedAt).Scan(&id))
	return id
}

func insertAdminPhoto(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, status string, size int64) {
	t.Helper()
	mustExecAdmin(t, pool,
		`INSERT INTO photos (event_id, storage_key, file_size, status) VALUES ($1, $2, $3, $4)`,
		eventID, "tenant/x/originals/"+uuid.NewString(), size, status)
}

func TestAdminRepository_Stats(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	userA := insertAdminUser(t, pool, "stats-a@example.com", 4096, false, base)
	userB := insertAdminUser(t, pool, "stats-b@example.com", 1024, true, base.Add(time.Minute))

	eventA := insertAdminEvent(t, pool, userA, "a", "a", false)
	insertAdminEvent(t, pool, userA, "deleted", "deleted", true)
	insertAdminEvent(t, pool, userB, "b", "b", false)

	insertAdminPhoto(t, pool, eventA, "READY", 100)
	insertAdminPhoto(t, pool, eventA, "PROCESSING", 200)
	insertAdminPhoto(t, pool, eventA, "FAILED", 400)
	insertAdminPhoto(t, pool, eventA, "UPLOADING", 800)

	mustExecAdmin(t, pool,
		`INSERT INTO invoices (user_id, amount_cents, status) VALUES ($1, 4900, 'paid'), ($1, 9900, 'open')`, userA)
	mustExecAdmin(t, pool,
		`INSERT INTO subscriptions (user_id, plan_id, status, current_period_end)
		 VALUES ($1, 'pro', 'active', NOW() + INTERVAL '10 days'),
		        ($1, 'pro', 'canceled', NULL),
		        ($2, 'free', 'trialing', NULL)`, userA, userB)

	stats, err := repo.Stats(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, stats.Users)
	require.EqualValues(t, 2, stats.Events, "soft-deleted events are excluded")
	require.EqualValues(t, 3, stats.Photos, "failed uploads are excluded")
	require.EqualValues(t, 5120, stats.StorageBytes)
	require.EqualValues(t, 4900, stats.RevenueCents, "only paid invoices count")
	require.EqualValues(t, 2, stats.Subscriptions, "canceled subscriptions are excluded")
}

func TestAdminRepository_ListUsers(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	older := insertAdminUser(t, pool, "older@example.com", 10, false, base)
	newer := insertAdminUser(t, pool, "newer@example.com", 20, true, base.Add(time.Hour))
	insertAdminEvent(t, pool, older, "one", "one", false)
	insertAdminEvent(t, pool, older, "two", "two", false)
	event := insertAdminEvent(t, pool, older, "deleted", "deleted", false)
	mustExecAdmin(t, pool, `UPDATE events SET deleted_at = NOW() WHERE id = $1`, event)

	page, err := repo.ListUsers(ctx, nil, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, newer, page[0].ID)
	require.True(t, page[0].IsAdmin)

	cursor := &admin.Cursor{CreatedAt: page[0].CreatedAt, ID: page[0].ID}
	page, err = repo.ListUsers(ctx, cursor, 10)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, older, page[0].ID)
	require.False(t, page[0].IsAdmin)
	require.EqualValues(t, 2, page[0].EventCount, "deleted events are not counted")
}

func TestAdminRepository_ListSubscriptions(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	user := insertAdminUser(t, pool, "subs@example.com", 0, false, base)
	periodEnd := base.Add(30 * 24 * time.Hour)

	mustExecAdmin(t, pool,
		`INSERT INTO subscriptions (user_id, plan_id, status, interval, current_period_end, created_at)
		 VALUES ($1, 'free', 'trialing', 'month', NULL, $2)`, user, base)
	mustExecAdmin(t, pool,
		`INSERT INTO subscriptions (user_id, plan_id, status, interval, current_period_end, created_at)
		 VALUES ($1, 'pro', 'active', 'year', $2, $3)`, user, periodEnd, base.Add(time.Hour))

	page, err := repo.ListSubscriptions(ctx, nil, 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	require.Equal(t, "pro", page[0].PlanID)
	require.Equal(t, "active", page[0].Status)
	require.Equal(t, "year", page[0].Interval)
	require.Equal(t, "subs@example.com", page[0].UserEmail)
	require.NotNil(t, page[0].CurrentPeriodEnd)
	require.Equal(t, "free", page[1].PlanID)
	require.Nil(t, page[1].CurrentPeriodEnd)
}

func TestAdminRepository_QueueHealth(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	oldest := time.Date(2026, 9, 14, 1, 0, 0, 0, time.UTC)
	mustExecAdmin(t, pool,
		`INSERT INTO jobs (type, status, run_at) VALUES
		 ('event.expire', 'pending', $1),
		 ('event.purge', 'pending', $2),
		 ('zip.generate', 'running', NOW()),
		 ('export.cleanup', 'failed', NOW())`, oldest, oldest.Add(time.Minute))

	health, err := repo.QueueHealth(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 2, health.Pending)
	require.EqualValues(t, 1, health.Running)
	require.EqualValues(t, 1, health.Failed)
	require.NotNil(t, health.OldestPendingAt)
	require.True(t, health.OldestPendingAt.Equal(oldest))
}

func TestAdminRepository_QueueHealth_Empty(t *testing.T) {
	pool := setupAdminDB(t)
	health, err := admin.NewRepository(pool).QueueHealth(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 0, health.Pending)
	require.Nil(t, health.OldestPendingAt)
}

func TestAdminRepository_IsAdmin(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	adminUser := insertAdminUser(t, pool, "admin@example.com", 0, true, base)
	plainUser := insertAdminUser(t, pool, "plain@example.com", 0, false, base)

	ok, err := repo.IsAdmin(ctx, adminUser)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.IsAdmin(ctx, plainUser)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = repo.IsAdmin(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, ok, "an unknown user is never an admin")
}

func TestAdminRepository_StorageTotals(t *testing.T) {
	pool := setupAdminDB(t)
	repo := admin.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	user := insertAdminUser(t, pool, "totals@example.com", 3000, false, base)
	event := insertAdminEvent(t, pool, user, "totals", "totals", false)
	insertAdminPhoto(t, pool, event, "READY", 100)
	insertAdminPhoto(t, pool, event, "FAILED", 200)
	insertAdminPhoto(t, pool, event, "UPLOADING", 400)

	totals, err := repo.StorageTotals(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 3000, totals.StorageBytes)
	require.EqualValues(t, 2, totals.Photos, "in-flight uploads have no counted original yet")
}
