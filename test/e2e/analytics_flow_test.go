//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/analytics"
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

func setupAnalyticsAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *r2.S3Store) {
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
	mustExec(t, pool, `CREATE TABLE event_analytics (
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		day DATE NOT NULL,
		gallery_views BIGINT NOT NULL DEFAULT 0,
		unique_visitors BIGINT NOT NULL DEFAULT 0,
		downloads BIGINT NOT NULL DEFAULT 0,
		qr_scans BIGINT NOT NULL DEFAULT 0,
		PRIMARY KEY (event_id, day)
	)`)
	mustExec(t, pool, `CREATE TABLE event_visitors (
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		day DATE NOT NULL,
		visitor_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (event_id, day, visitor_hash)
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
	authSvc := auth.NewService(userRepo, auth.NewRepository(pool),
		auth.NewTokenService("e2e-secret", 15*time.Minute, 30*24*time.Hour), &auth.LogMailer{}, auth.Config{
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

	photoRepo := photos.NewRepository(pool)
	analyticsSvc := analytics.NewService(analytics.NewRepository(pool), photoRepo, "e2e-salt", discardLoggerE2E())
	analyticsHandler := analytics.NewHandler(analyticsSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	gallerySvc := gallery.NewService(
		gallery.NewRepository(pool),
		photos.NewSignedURLGenerator(store, 5*time.Minute),
		gallery.NewUnlockTokens("e2e-secret", 30*time.Minute),
		auth.VerifyPassword,
		nil,
		analyticsSvc,
		nil)
	galleryHandler := gallery.NewHandler(gallerySvc)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Route("/public/events/{slug}", func(r chi.Router) {
			r.Get("/", galleryHandler.GetEvent)
			r.Post("/unlock", galleryHandler.Unlock)
			r.Get("/photos", galleryHandler.ListPhotos)
			r.Get("/photos/{photoID}", galleryHandler.GetPhoto)
			r.Get("/photos/{photoID}/url", galleryHandler.PhotoURL)
		})
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Post("/events", eventHandler.Create)
			r.Patch("/events/{eventID}/settings", eventHandler.UpdateSettings)
			r.Get("/events/{eventID}/analytics", analyticsHandler.Event)
			r.Get("/account/analytics", analyticsHandler.Account)
		})
	})
	return r, pool, store
}

func TestE2E_AnalyticsFlow(t *testing.T) {
	h, pool, store := setupAnalyticsAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"analytics@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Analytics Party"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	eventID := extractEventID(t, rec)

	slug := eventSlug(t, pool, eventID)
	photoID, _ := seedReadyPhoto(t, pool, store, slug)
	mustExec(t, pool, `UPDATE events SET photo_count = 1 WHERE id = '`+eventID+`'`)

	// Two views from the same visitor (same IP + user agent), one via QR from
	// another visitor.
	publicPath := "/api/v1/public/events/" + slug
	rec = doReqHeaders(t, h, http.MethodGet, publicPath, "", "", map[string]string{"X-Forwarded-For": "198.51.100.10"})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	rec = doReqHeaders(t, h, http.MethodGet, publicPath, "", "", map[string]string{"X-Forwarded-For": "198.51.100.10"})
	require.Equal(t, http.StatusOK, rec.Code)
	rec = doReqHeaders(t, h, http.MethodGet, publicPath+"?src=qr", "", "", map[string]string{"X-Forwarded-For": "198.51.100.20"})
	require.Equal(t, http.StatusOK, rec.Code)

	// An original download is recorded when the URL is issued.
	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+eventID+"/settings", `{"allowOriginalDownload":true}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	rec = doReq(t, h, http.MethodGet, publicPath+"/photos/"+photoID.String()+"/url?variant=original", "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	var eventEnv struct {
		Data struct {
			EventID    string `json:"eventId"`
			PhotoCount int64  `json:"photoCount"`
			Totals     struct {
				GalleryViews   int64 `json:"galleryViews"`
				UniqueVisitors int64 `json:"uniqueVisitors"`
				Downloads      int64 `json:"downloads"`
				QRScans        int64 `json:"qrScans"`
			} `json:"totals"`
			Daily []struct {
				Date           string `json:"date"`
				GalleryViews   int64  `json:"galleryViews"`
				UniqueVisitors int64  `json:"uniqueVisitors"`
			} `json:"daily"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/analytics?days=1", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &eventEnv))
	require.Equal(t, eventID, eventEnv.Data.EventID)
	require.EqualValues(t, 1, eventEnv.Data.PhotoCount)
	require.EqualValues(t, 3, eventEnv.Data.Totals.GalleryViews)
	require.EqualValues(t, 2, eventEnv.Data.Totals.UniqueVisitors)
	require.EqualValues(t, 1, eventEnv.Data.Totals.Downloads)
	require.EqualValues(t, 1, eventEnv.Data.Totals.QRScans)
	require.Len(t, eventEnv.Data.Daily, 1)
	require.Equal(t, time.Now().UTC().Format("2006-01-02"), eventEnv.Data.Daily[0].Date)
	require.EqualValues(t, 3, eventEnv.Data.Daily[0].GalleryViews)
	require.EqualValues(t, 2, eventEnv.Data.Daily[0].UniqueVisitors)

	// Account analytics aggregates the same counters and event counts.
	var accountEnv struct {
		Data struct {
			EventCount int64 `json:"eventCount"`
			PhotoCount int64 `json:"photoCount"`
			Totals     struct {
				GalleryViews int64 `json:"galleryViews"`
			} `json:"totals"`
			Daily []struct {
				Date string `json:"date"`
			} `json:"daily"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/account/analytics", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &accountEnv))
	require.EqualValues(t, 1, accountEnv.Data.EventCount)
	require.EqualValues(t, 1, accountEnv.Data.PhotoCount)
	require.EqualValues(t, 3, accountEnv.Data.Totals.GalleryViews)
	require.Len(t, accountEnv.Data.Daily, 1)

	// A second operator sees an empty account and no access to the event.
	rec = doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"other@example.com","password":"password123"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	otherAccess, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/analytics", "", otherAccess)
	require.Equal(t, http.StatusNotFound, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodGet, "/api/v1/account/analytics", "", otherAccess)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &accountEnv))
	require.EqualValues(t, 0, accountEnv.Data.EventCount)
	require.EqualValues(t, 0, accountEnv.Data.Totals.GalleryViews)
	require.Empty(t, accountEnv.Data.Daily)
}
