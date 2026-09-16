//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
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
	mustExec(t, pool, `CREATE TABLE refresh_tokens (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		family_id UUID NOT NULL,
		token_hash TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		revoked_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE INDEX idx_refresh_hash ON refresh_tokens(token_hash)`)
	mustExec(t, pool, `CREATE TABLE password_resets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	return pool
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

type memMailer struct{ sent []string }

func (m *memMailer) SendPasswordReset(ctx context.Context, toEmail, resetURL string) error {
	m.sent = append(m.sent, resetURL)
	return nil
}

func newService(t *testing.T, pool *pgxpool.Pool, mailer auth.Mailer) *auth.Service {
	t.Helper()
	userRepo := users.NewRepository(pool)
	authRepo := auth.NewRepository(pool)
	tokens := auth.NewTokenService("test-secret", 15*time.Minute, 30*24*time.Hour)
	return auth.NewService(userRepo, authRepo, tokens, mailer, auth.Config{
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       30 * 24 * time.Hour,
		PasswordResetTTL: time.Hour,
		LockoutMaxFailed: 3,
		LockoutDuration:  15 * time.Minute,
		PublicBaseURL:    "http://localhost:3000",
	})
}

func TestAuthFlow_SignupLoginRefreshLogout(t *testing.T) {
	pool := setupDB(t)
	svc := newService(t, pool, &memMailer{})
	ctx := context.Background()

	res, err := svc.Signup(ctx, "a@b.com", "password123", "Booth")
	require.NoError(t, err)
	require.NotEmpty(t, res.AccessToken)
	require.NotEmpty(t, res.RefreshToken)

	res2, err := svc.Login(ctx, "a@b.com", "password123")
	require.NoError(t, err)
	require.Equal(t, res.User.ID, res2.User.ID)

	rotated, err := svc.Refresh(ctx, res2.RefreshToken)
	require.NoError(t, err)
	require.NotEqual(t, res2.RefreshToken, rotated.RefreshToken)

	// Reuse of the old token revokes the family.
	_, err = svc.Refresh(ctx, res2.RefreshToken)
	require.Error(t, err)
	_, err = svc.Refresh(ctx, rotated.RefreshToken)
	require.Error(t, err)

	// Logout is safe even if already revoked.
	require.NoError(t, svc.Logout(ctx, rotated.RefreshToken))
}

func TestAuth_RefreshFamilyRevokedOnReuse(t *testing.T) {
	pool := setupDB(t)
	svc := newService(t, pool, &memMailer{})
	ctx := context.Background()

	res, err := svc.Signup(ctx, "a@b.com", "password123", "")
	require.NoError(t, err)

	rotated, err := svc.Refresh(ctx, res.RefreshToken)
	require.NoError(t, err)

	_, err = svc.Refresh(ctx, res.RefreshToken)
	require.Error(t, err)

	var revoked int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE revoked_at IS NULL`).Scan(&revoked)
	require.NoError(t, err)
	require.Equal(t, 0, revoked, "all tokens in family should be revoked")

	_, err = svc.Refresh(ctx, rotated.RefreshToken)
	require.Error(t, err)
}

func TestAuth_LockoutPersisted(t *testing.T) {
	pool := setupDB(t)
	svc := newService(t, pool, &memMailer{})
	ctx := context.Background()

	_, err := svc.Signup(ctx, "a@b.com", "password123", "")
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, _ = svc.Login(ctx, "a@b.com", "wrong")
	}

	var lockedUntil *time.Time
	err = pool.QueryRow(ctx, `SELECT locked_until FROM users WHERE email = 'a@b.com'`).Scan(&lockedUntil)
	require.NoError(t, err)
	require.NotNil(t, lockedUntil)

	_, err = svc.Login(ctx, "a@b.com", "password123")
	require.Error(t, err)
}

func TestAuth_PasswordResetSingleUseAndRevokesSessions(t *testing.T) {
	pool := setupDB(t)
	mailer := &memMailer{}
	svc := newService(t, pool, mailer)
	ctx := context.Background()

	res, err := svc.Signup(ctx, "a@b.com", "password123", "")
	require.NoError(t, err)

	require.NoError(t, svc.RequestPasswordReset(ctx, "a@b.com"))
	require.Len(t, mailer.sent, 1)
	token := extractToken(mailer.sent[0])

	require.NoError(t, svc.ConfirmPasswordReset(ctx, token, "newpassword123"))

	// All sessions revoked.
	var active int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, res.User.ID).Scan(&active)
	require.NoError(t, err)
	require.Equal(t, 0, active)

	// Single use.
	err = svc.ConfirmPasswordReset(ctx, token, "another123")
	require.Error(t, err)

	// New password works.
	_, err = svc.Login(ctx, "a@b.com", "newpassword123")
	require.NoError(t, err)
}

func TestAuth_CascadeDeleteOnUserRemoval(t *testing.T) {
	pool := setupDB(t)
	svc := newService(t, pool, &memMailer{})
	ctx := context.Background()

	res, err := svc.Signup(ctx, "a@b.com", "password123", "")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, res.User.ID)
	require.NoError(t, err)

	count := 0
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, res.User.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func extractToken(resetURL string) string {
	const marker = "token="
	for i := 0; i+len(marker) <= len(resetURL); i++ {
		if resetURL[i:i+len(marker)] == marker {
			return resetURL[i+len(marker):]
		}
	}
	return ""
}
