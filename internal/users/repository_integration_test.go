//go:build integration

package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) *pgxpool.Pool {
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

	mustExec(t, pool, `CREATE TABLE users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(320) NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		business_name VARCHAR(255),
		email_verified_at TIMESTAMPTZ,
		failed_login_count INT NOT NULL DEFAULT 0,
		locked_until TIMESTAMPTZ,
		is_admin BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return pool
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func TestRepository_CreateAndUniqueEmail(t *testing.T) {
	pool := setupDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	u, err := repo.Create(ctx, "a@b.com", "hash", "Booth")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, u.ID)

	_, err = repo.Create(ctx, "a@b.com", "hash", "")
	require.ErrorIs(t, err, users.ErrEmailTaken)
}

func TestRepository_GetByEmailAndID(t *testing.T) {
	pool := setupDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	created, err := repo.Create(ctx, "a@b.com", "hash", "Booth")
	require.NoError(t, err)

	byEmail, err := repo.GetByEmail(ctx, "a@b.com")
	require.NoError(t, err)
	require.Equal(t, created.ID, byEmail.ID)

	byID, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "Booth", byID.BusinessName)

	_, err = repo.GetByID(ctx, uuid.New())
	require.ErrorIs(t, err, users.ErrNotFound)
}

func TestRepository_ScansIsAdminFlag(t *testing.T) {
	pool := setupDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	u, err := repo.Create(ctx, "a@b.com", "hash", "Booth")
	require.NoError(t, err)
	require.False(t, u.IsAdmin)

	_, err = pool.Exec(ctx, `UPDATE users SET is_admin = TRUE WHERE id = $1`, u.ID)
	require.NoError(t, err)

	byID, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.True(t, byID.IsAdmin)

	byEmail, err := repo.GetByEmail(ctx, "a@b.com")
	require.NoError(t, err)
	require.True(t, byEmail.IsAdmin)
}

func TestRepository_LockoutCounterIncrementsAndResets(t *testing.T) {
	pool := setupDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	u, err := repo.Create(ctx, "a@b.com", "hash", "")
	require.NoError(t, err)

	lockUntil := time.Now().Add(15 * time.Minute)
	require.NoError(t, repo.RecordFailedLogin(ctx, u.ID, 3, lockUntil))
	require.NoError(t, repo.RecordFailedLogin(ctx, u.ID, 3, lockUntil))

	after, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 2, after.FailedLoginCount)
	require.Nil(t, after.LockedUntil)

	require.NoError(t, repo.RecordFailedLogin(ctx, u.ID, 3, lockUntil))
	locked, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, locked.LockedUntil)

	require.NoError(t, repo.ResetFailedLogin(ctx, u.ID))
	reset, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, 0, reset.FailedLoginCount)
	require.Nil(t, reset.LockedUntil)
}

func TestRepository_UpdateBusinessNameAndPassword(t *testing.T) {
	pool := setupDB(t)
	repo := users.NewRepository(pool)
	ctx := context.Background()

	u, err := repo.Create(ctx, "a@b.com", "hash", "Old")
	require.NoError(t, err)

	updated, err := repo.UpdateBusinessName(ctx, u.ID, "New")
	require.NoError(t, err)
	require.Equal(t, "New", updated.BusinessName)

	require.NoError(t, repo.UpdatePassword(ctx, u.ID, "newhash"))
	after, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "newhash", after.PasswordHash)
}
