//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/gallery"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/internal/qr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupGalleryAPI builds a full API (auth + events + settings + public gallery)
// backed by real Postgres and MinIO.
func setupGalleryAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *r2.S3Store) {
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
	mustExec(t, pool, `CREATE TABLE tenant_branding (
		user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
		business_name VARCHAR(255),
		logo_key TEXT,
		profile_image_key TEXT,
		primary_color VARCHAR(9),
		secondary_color VARCHAR(9),
		contact_email VARCHAR(320),
		contact_phone VARCHAR(40),
		website_url VARCHAR(320),
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
	qrSvc := qr.NewService(eventSvc, "http://localhost:3000")
	qrHandler := qr.NewHandler(qrSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})
	brandingSvc := users.NewBrandingService(userRepo, store, 5*time.Minute)
	brandingHandler := users.NewBrandingHandler(brandingSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	galleryRepo := gallery.NewRepository(pool)
	gallerySvc := gallery.NewService(galleryRepo,
		photos.NewSignedURLGenerator(store, 5*time.Minute),
		gallery.NewUnlockTokens("e2e-secret", 30*time.Minute),
		auth.VerifyPassword,
		brandingSvc)
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
			r.Get("/events/{eventID}/url", qrHandler.URL)
			r.Get("/events/{eventID}/qr.png", qrHandler.PNG)
			r.Get("/events/{eventID}/qr.svg", qrHandler.SVG)
			r.Get("/account/branding", brandingHandler.Get)
			r.Patch("/account/branding", brandingHandler.Update)
			r.Post("/account/branding/assets", brandingHandler.CreateAssetUpload)
		})
	})
	return r, pool, store
}

func seedReadyPhoto(t *testing.T, pool *pgxpool.Pool, store *r2.S3Store, slug string) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	var eventID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM events WHERE slug = $1`, slug).Scan(&eventID))

	photoID := uuid.New()
	optKey := "tenant/x/events/" + eventID.String() + "/optimized/" + photoID.String() + ".webp"
	require.NoError(t, store.Put(ctx, optKey, "image/webp", []byte("derivative")))

	_, err := pool.Exec(ctx,
		`INSERT INTO photos (id, event_id, storage_key, mime_type, file_size, status, thumbnail_key, medium_key, optimized_key, width, height)
		 VALUES ($1, $2, $3, 'image/jpeg', 10, 'READY', $4, $5, $6, 100, 100)`,
		photoID, eventID, "orig/"+photoID.String(), "thumb/"+photoID.String(), "med/"+photoID.String(), optKey)
	require.NoError(t, err)
	return photoID, optKey
}

func TestE2E_PublicGalleryFlow(t *testing.T) {
	h, pool, store := setupGalleryAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"gallery@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Public Gallery"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)

	slug := eventSlug(t, pool, eventID)
	photoID, _ := seedReadyPhoto(t, pool, store, slug)

	// Guest opens the event (no auth).
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug, "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))

	// Guest pages through photos (metadata only).
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos", "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotContains(t, rec.Body.String(), "thumbnailUrl")
	require.Contains(t, rec.Body.String(), photoID.String())

	// Guest opens the photo.
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos/"+photoID.String(), "", "")
	require.Equal(t, http.StatusOK, rec.Code)

	// Guest downloads the large variant via a signed URL, then fetches the bytes.
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos/"+photoID.String()+"/url?variant=large", "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))

	var urlEnv struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &urlEnv))
	require.NotEmpty(t, urlEnv.Data.URL)

	resp, err := http.Get(urlEnv.Data.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got, _ := io.ReadAll(resp.Body)
	require.Equal(t, "derivative", string(got))

	// Original download disabled by default.
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos/"+photoID.String()+"/url?variant=original", "", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "ORIGINAL_DOWNLOAD_DISABLED")

	// View-only event: rendering variants stay available, downloads do not.
	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+eventID+"/settings", `{"allowDownload":false}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos/"+photoID.String()+"/url?variant=large", "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos/"+photoID.String()+"/url?variant=original", "", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Contains(t, rec.Body.String(), "DOWNLOAD_DISABLED")
}

func TestE2E_PasswordGalleryFlow(t *testing.T) {
	h, pool, store := setupGalleryAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"pw@example.com","password":"password123"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Password Gallery"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)

	// Configure a gallery password.
	rec = doReq(t, h, http.MethodPatch, "/api/v1/events/"+eventID+"/settings",
		`{"visibility":"password","password":"open-sesame"}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	slug := eventSlug(t, pool, eventID)
	seedReadyPhoto(t, pool, store, slug)

	// Without unlock -> 401.
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos", "", "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Wrong password -> 401.
	rec = doReq(t, h, http.MethodPost, "/api/v1/public/events/"+slug+"/unlock", `{"password":"wrong"}`, "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// Correct password -> token.
	rec = doReq(t, h, http.MethodPost, "/api/v1/public/events/"+slug+"/unlock", `{"password":"open-sesame"}`, "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var unlockEnv struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &unlockEnv))
	require.NotEmpty(t, unlockEnv.Data.Token)

	// With token -> 200.
	rec = doReqHeaders(t, h, http.MethodGet, "/api/v1/public/events/"+slug+"/photos", "",
		"", map[string]string{"X-Gallery-Unlock": unlockEnv.Data.Token})
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
}

func eventSlug(t *testing.T, pool *pgxpool.Pool, eventID string) string {
	t.Helper()
	var slug string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT slug FROM events WHERE id = $1`, eventID).Scan(&slug))
	return slug
}
