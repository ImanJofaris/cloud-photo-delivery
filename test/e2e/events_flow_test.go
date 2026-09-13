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
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupEventsAPI(t *testing.T) http.Handler {
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
	mustExec(t, pool, `CREATE TABLE password_resets (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash TEXT NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL,
		used_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL,
		client_name VARCHAR(255),
		client_email VARCHAR(320),
		location VARCHAR(255),
		description TEXT,
		event_date DATE,
		status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
		cover_photo_id UUID,
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		photo_count BIGINT NOT NULL DEFAULT 0,
		guest_count BIGINT NOT NULL DEFAULT 0,
		expires_at TIMESTAMPTZ,
		expiry_warned_at TIMESTAMPTZ,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT events_slug_unique UNIQUE (user_id, slug)
	)`)
	mustExec(t, pool, `CREATE TABLE event_settings (
		event_id UUID PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
		visibility VARCHAR(30) NOT NULL DEFAULT 'public',
		password_hash TEXT,
		allow_download BOOLEAN NOT NULL DEFAULT TRUE,
		allow_original_download BOOLEAN NOT NULL DEFAULT FALSE,
		watermark_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		password_changed_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	userRepo := users.NewRepository(pool)
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

	eventRepo := events.NewRepository(pool)
	eventSvc := events.NewService(eventRepo, limits.NewDefault(), auth.HashPassword)
	eventHandler := events.NewHandler(eventSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Route("/events", func(r chi.Router) {
				r.Post("/", eventHandler.Create)
				r.Get("/", eventHandler.List)
				r.Route("/{eventID}", func(r chi.Router) {
					r.Get("/", eventHandler.Get)
					r.Patch("/", eventHandler.Update)
					r.Post("/archive", eventHandler.Archive)
					r.Delete("/", eventHandler.Delete)
					r.Get("/settings", eventHandler.GetSettings)
					r.Patch("/settings", eventHandler.UpdateSettings)
				})
			})
		})
	})
	return r
}

func doReq(t *testing.T, h http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body == "" {
		rdr = bytes.NewBuffer(nil)
	} else {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func extractEventID(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var env struct {
		Data struct {
			Event struct {
				ID string `json:"id"`
			} `json:"event"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.NotEmpty(t, env.Data.Event.ID, "body=%s", rec.Body.String())
	return env.Data.Event.ID
}

func TestE2E_EventsLifecycle(t *testing.T) {
	h := setupEventsAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"ops@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	// create 3 events
	var ids []string
	for i, name := range []string{"Summer Party", "Corporate Gala", "Wedding Expo"} {
		rec = doReq(t, h, http.MethodPost, "/api/v1/events",
			`{"name":"`+name+`","clientEmail":"client@example.com"}`, access)
		require.Equal(t, http.StatusCreated, rec.Code, "create %d body=%s", i, rec.Body.String())
		ids = append(ids, extractEventID(t, rec))
	}

	// list all
	rec = doReq(t, h, http.MethodGet, "/api/v1/events", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Summer Party")
	require.Contains(t, rec.Body.String(), "Corporate Gala")
	require.Contains(t, rec.Body.String(), "Wedding Expo")

	// filter by status=upcoming
	rec = doReq(t, h, http.MethodGet, "/api/v1/events?status=upcoming", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "items")

	// update one
	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+ids[0],
		`{"name":"Summer Party 2026"}`, access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Summer Party 2026")

	// archive another
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+ids[1]+"/archive", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "archived")

	rec = doReq(t, h, http.MethodGet, "/api/v1/events?status=archived", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "Corporate Gala")

	// settings
	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+ids[2]+"/settings",
		`{"visibility":"password","password":"guestpass"}`, access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "passwordProtected")

	// delete one
	rec = doReq(t, h, http.MethodDelete, "/api/v1/events/"+ids[0], "", access)
	require.Equal(t, http.StatusNoContent, rec.Code)

	// list excludes deleted
	rec = doReq(t, h, http.MethodGet, "/api/v1/events", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "Summer Party 2026")
}

func TestE2E_EventsTenantIsolation(t *testing.T) {
	h := setupEventsAPI(t)

	recA := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"a@example.com","password":"password123"}`, "")
	accessA, _ := parseAuth(t, recA)
	recB := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"b@example.com","password":"password123"}`, "")
	accessB, _ := parseAuth(t, recB)

	rec := doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"A Private Event"}`, accessA)
	require.Equal(t, http.StatusCreated, rec.Code)
	id := extractEventID(t, rec)

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+id, "", accessB)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")

	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+id, `{"name":"Hacked"}`, accessB)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = doReq(t, h, http.MethodDelete, "/api/v1/events/"+id, "", accessB)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
