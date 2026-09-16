//go:build integration

package e2e_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/exports"
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

func setupExportsAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *r2.S3Store) {
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
	mustExec(t, pool, `CREATE TABLE jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type VARCHAR(50) NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}'::jsonb,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		attempts INT NOT NULL DEFAULT 0,
		max_attempts INT NOT NULL DEFAULT 5,
		last_error TEXT,
		locked_at TIMESTAMPTZ,
		locked_by TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT jobs_status_check CHECK (status IN ('pending','running','done','failed'))
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

	photoRepo := photos.NewRepository(pool)
	exportRepo := exports.NewRepository(pool)
	exportSvc := exports.NewService(exportRepo, photoRepo, photoRepo,
		exports.NewPostgresQueue(pool), store, time.Hour)
	exportHandler := exports.NewHandler(exportSvc, func(r *http.Request) (string, bool) {
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
				r.Route("/{eventID}", func(r chi.Router) {
					r.Post("/exports", exportHandler.Create)
					r.Get("/exports/{exportID}", exportHandler.Get)
				})
			})
		})
	})
	return r, pool, store
}

func TestE2E_BulkZipExportFlow(t *testing.T) {
	h, pool, store := setupExportsAPI(t)
	ctx := context.Background()

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"zip@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Export Party"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	var createEnv struct {
		Data struct {
			Event struct {
				ID string `json:"id"`
			} `json:"event"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createEnv), "body=%s", rec.Body.String())
	eventID := createEnv.Data.Event.ID
	require.NotEmpty(t, eventID)

	// Seed two READY photos with their original objects, emulating uploads.
	photoA, photoB := uuid.New(), uuid.New()
	keyA := "tenant/" + eventID + "/originals/" + photoA.String() + "/a.jpg"
	keyB := "tenant/" + eventID + "/originals/" + photoB.String() + "/b.jpg"
	mustExec(t, pool, `INSERT INTO photos (id, event_id, storage_key, original_filename, mime_type, file_size, status, created_at, updated_at)
		VALUES ('`+photoA.String()+`', '`+eventID+`', '`+keyA+`', 'a.jpg', 'image/jpeg', 3, 'READY', NOW(), NOW()),
		       ('`+photoB.String()+`', '`+eventID+`', '`+keyB+`', 'b.jpg', 'image/jpeg', 3, 'READY', NOW() - INTERVAL '1 second', NOW())`)
	require.NoError(t, store.Put(ctx, keyA, "image/jpeg", []byte("aaa")))
	require.NoError(t, store.Put(ctx, keyB, "image/jpeg", []byte("bbb")))

	// POST starts the export and queues zip.generate.
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/exports", "", access)
	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())
	var exportEnv struct {
		Data struct {
			ID          string  `json:"id"`
			Status      string  `json:"status"`
			DownloadURL *string `json:"downloadUrl"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exportEnv))
	require.Equal(t, "pending", exportEnv.Data.Status)
	require.Nil(t, exportEnv.Data.DownloadURL)
	exportID := exportEnv.Data.ID
	require.NotEmpty(t, exportID)

	var queued int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM jobs WHERE type = 'zip.generate' AND payload->>'exportId' = $1`, exportID).Scan(&queued))
	require.Equal(t, 1, queued, "export request must enqueue zip.generate")

	// Reposting while pending reuses the active export.
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/exports", "", access)
	require.Equal(t, http.StatusAccepted, rec.Code)
	require.Contains(t, rec.Body.String(), exportID, "active export should be reused")

	// Run the worker handler in-process, then poll the status endpoint.
	exportRepo := exports.NewRepository(pool)
	photoRepo := photos.NewRepository(pool)
	payload, err := json.Marshal(exports.GeneratePayload{ExportID: uuid.MustParse(exportID), EventID: uuid.MustParse(eventID)})
	require.NoError(t, err)
	generate := exports.GenerateHandler(exportRepo, photoRepo.EventOwner, photoRepo, store, time.Hour, discardLoggerE2E())
	require.NoError(t, generate(ctx, payload))

	var downloadURL string
	for i := 0; i < 10; i++ {
		rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/exports/"+exportID, "", access)
		require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exportEnv))
		if exportEnv.Data.Status == "ready" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.Equal(t, "ready", exportEnv.Data.Status, "body=%s", rec.Body.String())
	require.NotNil(t, exportEnv.Data.DownloadURL)
	downloadURL = *exportEnv.Data.DownloadURL

	// The signed URL downloads a ZIP containing both originals.
	resp, err := http.Get(downloadURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	require.Len(t, zr.File, 2)
	entries := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		entries[f.Name] = string(body)
	}
	require.Equal(t, "aaa", entries["a.jpg"])
	require.Equal(t, "bbb", entries["b.jpg"])

	// Expiry: cleanup deletes the object, keeps the row, and the URL is gone.
	mustExec(t, pool, `UPDATE exports SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = '`+exportID+`'`)
	cleanup := exports.CleanupHandler(exportRepo, store, discardLoggerE2E())
	require.NoError(t, cleanup(ctx, nil))

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/exports/"+exportID, "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &exportEnv))
	require.Equal(t, "expired", exportEnv.Data.Status)
	require.Nil(t, exportEnv.Data.DownloadURL)

	_, err = store.Head(ctx, "tenant/"+eventID+"/exports/"+exportID+".zip")
	require.True(t, errors.Is(err, r2.ErrNotFound), "archive object should be deleted, got %v", err)
}
