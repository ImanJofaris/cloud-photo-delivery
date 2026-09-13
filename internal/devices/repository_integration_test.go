//go:build integration

package devices_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/devices"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
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
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT events_slug_unique UNIQUE (user_id, slug)
	)`)
	mustExec(t, pool, `CREATE TABLE devices (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(120) NOT NULL,
		key_prefix VARCHAR(12) NOT NULL,
		key_hash TEXT NOT NULL,
		assigned_event_id UUID REFERENCES events(id) ON DELETE SET NULL,
		revoked_at TIMESTAMPTZ,
		last_used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE INDEX idx_devices_user ON devices(user_id)`)
	mustExec(t, pool, `CREATE UNIQUE INDEX idx_devices_prefix ON devices(key_prefix)`)

	var userID uuid.UUID
	err = pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('a@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users`)
	})
	return pool, userID
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func seedUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

func seedEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (user_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		userID, "Event "+slug, slug).Scan(&id)
	require.NoError(t, err)
	return id
}

func newDevice(userID uuid.UUID, name string, assigned *uuid.UUID) (*devices.Device, string) {
	raw, prefix, hash, _ := devices.GenerateKey()
	return &devices.Device{
		ID:              uuid.New(),
		UserID:          userID,
		Name:            name,
		KeyPrefix:       prefix,
		KeyHash:         hash,
		AssignedEventID: assigned,
		CreatedAt:       time.Now().UTC(),
	}, raw
}

func TestRepository_CreateAndList(t *testing.T) {
	pool, userID := setupDB(t)
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	eventID := seedEvent(t, pool, userID, "wedding")
	d, raw := newDevice(userID, "Booth 1", &eventID)
	created, err := repo.Create(ctx, d)
	require.NoError(t, err)
	require.Equal(t, d.ID, created.ID)

	items, err := repo.ListByUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Booth 1", items[0].Name)
	require.Equal(t, devices.HashKey(raw), items[0].KeyHash)
	require.NotEqual(t, raw, items[0].KeyHash)
	require.NotNil(t, items[0].AssignedEventID)
	require.Equal(t, eventID, *items[0].AssignedEventID)
}

func TestRepository_TenantIsolation(t *testing.T) {
	pool, userA := setupDB(t)
	userB := seedUser(t, pool, "b@example.com")
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	d, _ := newDevice(userA, "A Booth", nil)
	_, err := repo.Create(ctx, d)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, userB, d.ID)
	require.ErrorIs(t, err, devices.ErrNotFound)
	_, err = repo.Rename(ctx, userB, d.ID, "Hacked")
	require.ErrorIs(t, err, devices.ErrNotFound)
	err = repo.Revoke(ctx, userB, d.ID, time.Now())
	require.ErrorIs(t, err, devices.ErrNotFound)

	items, err := repo.ListByUser(ctx, userB)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestRepository_GetByPrefixAndRotate(t *testing.T) {
	pool, userID := setupDB(t)
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	d, raw := newDevice(userID, "Booth", nil)
	_, err := repo.Create(ctx, d)
	require.NoError(t, err)

	found, err := repo.GetByPrefix(ctx, d.KeyPrefix)
	require.NoError(t, err)
	require.Equal(t, d.ID, found.ID)

	newRaw, newPrefix, newHash, err := devices.GenerateKey()
	require.NoError(t, err)
	rotated, err := repo.Rotate(ctx, userID, d.ID, newPrefix, newHash)
	require.NoError(t, err)
	require.Equal(t, newPrefix, rotated.KeyPrefix)

	_, err = repo.GetByPrefix(ctx, d.KeyPrefix)
	require.ErrorIs(t, err, devices.ErrNotFound)
	updated, err := repo.GetByPrefix(ctx, newPrefix)
	require.NoError(t, err)
	require.True(t, devices.VerifyKey(newRaw, updated.KeyHash))
	require.False(t, devices.VerifyKey(raw, updated.KeyHash))
}

func TestRepository_RevokeAndTouch(t *testing.T) {
	pool, userID := setupDB(t)
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	d, _ := newDevice(userID, "Booth", nil)
	_, err := repo.Create(ctx, d)
	require.NoError(t, err)

	at := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.TouchLastUsed(ctx, d.ID, at))

	stored, err := repo.GetByID(ctx, userID, d.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.LastUsedAt)
	require.WithinDuration(t, at, *stored.LastUsedAt, time.Microsecond)

	revokedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, repo.Revoke(ctx, userID, d.ID, revokedAt))
	stored, err = repo.GetByID(ctx, userID, d.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.RevokedAt)
	require.True(t, stored.Revoked())
}

func TestRepository_CascadeOnUserDelete(t *testing.T) {
	pool, userID := setupDB(t)
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	d, _ := newDevice(userID, "Booth", nil)
	_, err := repo.Create(ctx, d)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM devices WHERE id = $1`, d.ID).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestRepository_AssignedEventSetNullOnEventDelete(t *testing.T) {
	pool, userID := setupDB(t)
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	eventID := seedEvent(t, pool, userID, "doomed")
	d, _ := newDevice(userID, "Booth", &eventID)
	_, err := repo.Create(ctx, d)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	require.NoError(t, err)

	stored, err := repo.GetByID(ctx, userID, d.ID)
	require.NoError(t, err)
	require.Nil(t, stored.AssignedEventID)
}

func TestRepository_EventOwnedBy(t *testing.T) {
	pool, userA := setupDB(t)
	userB := seedUser(t, pool, "b@example.com")
	repo := devices.NewRepository(pool)
	ctx := context.Background()

	eventA := seedEvent(t, pool, userA, "a")
	require.True(t, mustOwned(t, repo, ctx, userA, eventA))
	require.False(t, mustOwned(t, repo, ctx, userB, eventA))
	require.False(t, mustOwned(t, repo, ctx, userA, uuid.New()))

	_, err := pool.Exec(ctx, `UPDATE events SET deleted_at = NOW() WHERE id = $1`, eventA)
	require.NoError(t, err)
	require.False(t, mustOwned(t, repo, ctx, userA, eventA))
}

func mustOwned(t *testing.T, repo *devices.PostgresRepository, ctx context.Context, userID, eventID uuid.UUID) bool {
	t.Helper()
	owned, err := repo.EventOwnedBy(ctx, userID, eventID)
	require.NoError(t, err)
	return owned
}
