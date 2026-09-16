//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/gallery"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func discardLoggerE2E() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type e2eBrandingProvider struct{}

func (e2eBrandingProvider) Get(ctx context.Context, userID uuid.UUID) (*users.BrandingView, error) {
	return nil, nil
}

type fakeLifecycleNotifier struct {
	warned []events.ExpiryWarning
}

func (f *fakeLifecycleNotifier) NotifyExpiryWarning(ctx context.Context, w events.ExpiryWarning) error {
	f.warned = append(f.warned, w)
	return nil
}

func setupLifecycleAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *r2.S3Store, *fakeLifecycleNotifier) {
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
		storage_bytes BIGINT NOT NULL DEFAULT 0,
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
		CONSTRAINT events_status_check CHECK (status IN ('upcoming','active','completed','archived','expired')),
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
	mustExec(t, pool, `CREATE TABLE photos (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		storage_key TEXT NOT NULL,
		thumbnail_key TEXT,
		optimized_key TEXT,
		medium_key TEXT,
		original_filename TEXT,
		mime_type VARCHAR(100) NOT NULL,
		file_size BIGINT NOT NULL,
		width INT,
		height INT,
		status VARCHAR(30) NOT NULL DEFAULT 'UPLOADING',
		upload_kind VARCHAR(20) NOT NULL DEFAULT 'simple',
		multipart_upload_id TEXT,
		error_message TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE exports (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL,
		object_key TEXT,
		file_size BIGINT,
		error_message TEXT,
		expires_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	endpoint := startMinioE2E(t)
	createBucketE2E(t, endpoint, "cpd-photos")
	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
		UseSSL:    false,
	})
	require.NoError(t, err)

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

	galleryRepo := gallery.NewRepository(pool)
	urls := photos.NewSignedURLGenerator(store, 5*time.Minute)
	tokens := gallery.NewUnlockTokens("e2e-secret", 30*time.Minute)
	gallerySvc := gallery.NewService(galleryRepo, urls, tokens, auth.VerifyPassword, e2eBrandingProvider{}, nil)
	galleryHandler := gallery.NewHandler(gallerySvc)

	notifier := &fakeLifecycleNotifier{}

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Route("/public/events/{slug}", func(r chi.Router) {
			r.Get("/", galleryHandler.GetEvent)
		})
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Route("/events", func(r chi.Router) {
				r.Post("/", eventHandler.Create)
				r.Route("/{eventID}", func(r chi.Router) {
					r.Get("/", eventHandler.Get)
					r.Post("/extend", eventHandler.Extend)
				})
			})
		})
	})
	return r, pool, store, notifier
}

func lifecycleEvent(t *testing.T, rec *httptest.ResponseRecorder) (string, string) {
	t.Helper()
	var env struct {
		Data struct {
			Event struct {
				ID        string `json:"id"`
				Slug      string `json:"slug"`
				Status    string `json:"status"`
				ExpiresAt string `json:"expiresAt"`
			} `json:"event"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "body=%s", rec.Body.String())
	require.NotEmpty(t, env.Data.Event.ID)
	return env.Data.Event.ID, env.Data.Event.Slug
}

func TestE2E_EventLifecycleExpireExtendPurge(t *testing.T) {
	h, pool, store, notifier := setupLifecycleAPI(t)
	ctx := context.Background()

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"life@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	expires := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	rec = doReq(t, h, http.MethodPost, "/api/v1/events",
		`{"name":"Short Retention","expiresAt":"`+expires+`"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	eventID, slug := lifecycleEvent(t, rec)

	// Seed a ready photo and its R2 objects, emulating a completed upload.
	photoID := uuid.New()
	originalKey := "tenant/" + eventID + "/photo-original.jpg"
	mustExec(t, pool, `INSERT INTO photos (id, event_id, storage_key, mime_type, file_size, status)
		VALUES ('`+photoID.String()+`', '`+eventID+`', '`+originalKey+`', 'image/jpeg', 4, 'READY')`)
	require.NoError(t, store.Put(ctx, originalKey, "image/jpeg", []byte("jpeg")))
	mustExec(t, pool, `UPDATE events SET storage_bytes = 4, photo_count = 1 WHERE id = '`+eventID+`'`)
	mustExec(t, pool, `UPDATE users SET storage_bytes = 4 WHERE email = 'life@example.com'`)

	// Gallery is reachable before expiry.
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug, "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	// Expire: the worker marks the event; public access is revoked.
	expire := events.ExpireHandler(events.NewRepository(pool), notifier, 7*24*time.Hour, time.Now, discardLoggerE2E())
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = '`+eventID+`'`)
	require.NoError(t, expire(ctx, nil))

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID, "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"expired"`)

	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug, "", "")
	require.Equal(t, http.StatusNotFound, rec.Code)

	// Extend: the event re-activates and the gallery is reachable again.
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/extend", `{"days":1}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"status":"active"`)

	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug, "", "")
	require.Equal(t, http.StatusOK, rec.Code)

	// Purge: with the expiry past the grace period the worker deletes objects
	// and rows, and storage returns to zero.
	mustExec(t, pool, `UPDATE events SET status = 'expired', expires_at = NOW() - INTERVAL '40 days' WHERE id = '`+eventID+`'`)
	require.NoError(t, expire(ctx, nil))

	purge := events.PurgeHandler(events.NewRepository(pool), store, 30*24*time.Hour, time.Now, discardLoggerE2E())
	require.NoError(t, purge(ctx, nil))

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID, "", access)
	require.Equal(t, http.StatusNotFound, rec.Code)

	_, err := store.Head(ctx, originalKey)
	require.True(t, errors.Is(err, r2.ErrNotFound), "R2 object should be gone, got %v", err)

	var storage int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE email = 'life@example.com'`).Scan(&storage))
	require.Zero(t, storage)
}
