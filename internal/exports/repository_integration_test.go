//go:build integration

package exports_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/exports"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func setupExportsDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
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

	mustExec(t, pool, `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`)
	mustExec(t, pool, `CREATE TABLE users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(320) NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL,
		status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		photo_count BIGINT NOT NULL DEFAULT 0,
		guest_count BIGINT NOT NULL DEFAULT 0,
		expires_at TIMESTAMPTZ,
		expiry_warned_at TIMESTAMPTZ,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE exports (
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
	)`)

	userA := insertUser(t, pool, "a@example.com")
	userB := insertUser(t, pool, "b@example.com")
	eventA := insertEvent(t, pool, userA, "wedding-a", "wedding-a")
	eventB := insertEvent(t, pool, userB, "wedding-b", "wedding-b")
	return pool, userA, userB, eventA, eventB
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&id))
	return id
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO events (user_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, slug).Scan(&id))
	return id
}

func TestExportsRepository_Lifecycle(t *testing.T) {
	pool, userA, _, eventA, _ := setupExportsDB(t)
	repo := exports.NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	require.Equal(t, exports.StatusPending, created.Status)
	require.NotEqual(t, uuid.Nil, created.ID)

	active, err := repo.GetActiveByEvent(ctx, eventA)
	require.NoError(t, err)
	require.Equal(t, created.ID, active.ID)

	require.NoError(t, repo.MarkProcessing(ctx, created.ID))
	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, exports.StatusProcessing, got.Status)

	expires := time.Now().UTC().Add(24 * time.Hour)
	require.NoError(t, repo.MarkReady(ctx, created.ID, "tenant/x/exports/e.zip", 4096, expires))
	got, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, exports.StatusReady, got.Status)
	require.NotNil(t, got.ObjectKey)
	require.Equal(t, "tenant/x/exports/e.zip", *got.ObjectKey)
	require.NotNil(t, got.FileSize)
	require.EqualValues(t, 4096, *got.FileSize)

	_, err = repo.GetActiveByEvent(ctx, eventA)
	require.ErrorIs(t, err, exports.ErrNotFound, "ready exports are not active")

	owned, err := repo.GetOwned(ctx, userA, eventA, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, owned.ID)

	_, err = repo.GetOwned(ctx, uuid.New(), eventA, created.ID)
	require.ErrorIs(t, err, exports.ErrNotFound)

	require.NoError(t, repo.MarkExpired(ctx, created.ID))
	got, err = repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, exports.StatusExpired, got.Status)
}

func TestExportsRepository_TenantIsolation(t *testing.T) {
	pool, userA, userB, _, eventB := setupExportsDB(t)
	repo := exports.NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, eventB)
	require.NoError(t, err)

	_, err = repo.GetOwned(ctx, userA, eventB, created.ID)
	require.ErrorIs(t, err, exports.ErrNotFound, "other tenants must not see the export")

	got, err := repo.GetOwned(ctx, userB, eventB, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	_, err = repo.GetOwned(ctx, userB, uuid.New(), created.ID)
	require.ErrorIs(t, err, exports.ErrNotFound, "wrong event must not resolve")
}

func TestExportsRepository_SoftDeletedEventHidden(t *testing.T) {
	pool, userA, _, eventA, _ := setupExportsDB(t)
	repo := exports.NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	mustExec(t, pool, `UPDATE events SET deleted_at = NOW() WHERE id = $1`, eventA)

	_, err = repo.GetOwned(ctx, userA, eventA, created.ID)
	require.ErrorIs(t, err, exports.ErrNotFound)
}

func TestExportsRepository_ListExpiredAndStaleProcessing(t *testing.T) {
	pool, _, _, eventA, _ := setupExportsDB(t)
	repo := exports.NewRepository(pool)
	ctx := context.Background()

	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	expired, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	require.NoError(t, repo.MarkReady(ctx, expired.ID, "expired.zip", 10, past))

	live, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	require.NoError(t, repo.MarkReady(ctx, live.ID, "live.zip", 10, future))

	expiredList, err := repo.ListExpired(ctx, time.Now().UTC(), 10)
	require.NoError(t, err)
	require.Len(t, expiredList, 1)
	require.Equal(t, expired.ID, expiredList[0].ID)

	stale, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	require.NoError(t, repo.MarkProcessing(ctx, stale.ID))
	fresh, err := repo.Create(ctx, eventA)
	require.NoError(t, err)
	require.NoError(t, repo.MarkProcessing(ctx, fresh.ID))

	mustExec(t, pool, `UPDATE exports SET updated_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, stale.ID)

	staleList, err := repo.ListStaleProcessing(ctx, time.Now().UTC().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, staleList, 1)
	require.Equal(t, stale.ID, staleList[0].ID)

	require.NoError(t, repo.MarkFailed(ctx, stale.ID, "export timed out"))
	got, err := repo.GetByID(ctx, stale.ID)
	require.NoError(t, err)
	require.Equal(t, exports.StatusFailed, got.Status)
	require.NotNil(t, got.ErrorMessage)
	require.Equal(t, "export timed out", *got.ErrorMessage)
}

func TestExportsRepository_StatusCheckEnforced(t *testing.T) {
	pool, _, _, eventA, _ := setupExportsDB(t)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO exports (id, event_id, status) VALUES ($1, $2, 'bogus')`, uuid.New(), eventA)
	require.Error(t, err, "unknown status must be rejected by the CHECK constraint")
}
