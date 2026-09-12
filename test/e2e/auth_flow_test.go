//go:build integration

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setup(t *testing.T) (*pgxpool.Pool, http.Handler) {
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

	userRepo := users.NewRepository(pool)
	userSvc := users.NewService(userRepo)
	authRepo := auth.NewRepository(pool)
	tokenSvc := auth.NewTokenService("e2e-secret", 15*time.Minute, 30*24*time.Hour)
	authSvc := auth.NewService(userRepo, authRepo, tokenSvc, &auth.LogMailer{}, auth.Config{
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       30 * 24 * time.Hour,
		PasswordResetTTL: time.Hour,
		LockoutMaxFailed: 5,
		LockoutDuration:  15 * time.Minute,
		PublicBaseURL:    "http://localhost:3000",
	})
	authHandler := auth.NewHandler(authSvc)
	userHandler := users.NewHandler(userSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	log := logging.New("test", "error")
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req)
		})
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/refresh", authHandler.Refresh)
		r.Post("/auth/logout", authHandler.Logout)
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Get("/account/me", userHandler.Me)
			r.Patch("/account/me", userHandler.UpdateMe)
		})
	})
	_ = log

	return pool, r
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func post(t *testing.T, h http.Handler, path, body string, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func get(t *testing.T, h http.Handler, path, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func parseAuth(t *testing.T, rec *httptest.ResponseRecorder) (access, refresh string) {
	t.Helper()
	var env struct {
		Data struct {
			AccessToken  string `json:"accessToken"`
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env.Data.AccessToken, env.Data.RefreshToken
}

func TestE2E_SignupLoginMeRefreshLogout(t *testing.T) {
	_, h := setup(t)

	rec := post(t, h, "/api/v1/auth/signup",
		`{"email":"a@b.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	_, refresh := parseAuth(t, rec)

	rec = post(t, h, "/api/v1/auth/login", `{"email":"a@b.com","password":"password123"}`, "")
	require.Equal(t, http.StatusOK, rec.Code)
	access, refresh := parseAuth(t, rec)

	rec = get(t, h, "/api/v1/account/me", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "a@b.com")

	rec = post(t, h, "/api/v1/auth/refresh", `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, http.StatusOK, rec.Code)
	newAccess, newRefresh := parseAuth(t, rec)
	require.NotEqual(t, refresh, newRefresh)

	rec = post(t, h, "/api/v1/auth/logout", `{"refreshToken":"`+newRefresh+`"}`, newAccess)
	require.Equal(t, http.StatusOK, rec.Code)

	// Refresh after logout must fail.
	rec = post(t, h, "/api/v1/auth/refresh", `{"refreshToken":"`+newRefresh+`"}`, "")
	require.NotEqual(t, http.StatusOK, rec.Code)
}

func TestE2E_ProtectedRouteRejectsBadToken(t *testing.T) {
	_, h := setup(t)

	rec := get(t, h, "/api/v1/account/me", "not-a-token")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = get(t, h, "/api/v1/account/me", "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
