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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/devices"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/internal/uploads"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startMinioE2E(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "quay.io/minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	host, err := c.Host(ctx)
	require.NoError(t, err)
	port, err := c.MappedPort(ctx, "9000")
	require.NoError(t, err)
	return "http://" + host + ":" + port.Port()
}

func createBucketE2E(t *testing.T, endpoint, bucket string) {
	t.Helper()
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	require.NoError(t, err)
	client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	_, err = client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	require.NoError(t, err)
}

func setupUploadsAPI(t *testing.T) (http.Handler, *pgxpool.Pool) {
	h, pool, _ := setupUploadsAPIWithEndpoint(t)
	return h, pool
}

func setupUploadsAPIWithEndpoint(t *testing.T) (http.Handler, *pgxpool.Pool, string) {
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
	mustExec(t, pool, `CREATE TABLE upload_idempotency (
		idempotency_key TEXT PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
		request_hash TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE upload_parts (
		photo_id UUID NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
		part_number INT NOT NULL,
		etag TEXT,
		PRIMARY KEY (photo_id, part_number)
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
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	mustExec(t, pool, `CREATE UNIQUE INDEX idx_devices_prefix ON devices(key_prefix)`)

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
	uploadRepo := uploads.NewRepository(pool)
	uploadSvc := uploads.NewService(photoRepo, uploadRepo, store, uploads.NewPostgresQueue(pool), limits.NewDefault())
	uploadHandler := uploads.NewHandler(uploadSvc, func(r *http.Request) (uploads.Actor, bool) {
		if device, ok := devices.FromContext(r.Context()); ok {
			return uploads.Actor{
				UserID:        device.UserID,
				DeviceID:      device.ID,
				AssignedEvent: device.AssignedEventID,
			}, true
		}
		id, ok := auth.UserID(r.Context())
		if !ok {
			return uploads.Actor{}, false
		}
		return uploads.Actor{UserID: id}, true
	})

	deviceRepo := devices.NewRepository(pool)
	deviceSvc := devices.NewService(deviceRepo)
	deviceHandler := devices.NewHandler(deviceSvc, func(r *http.Request) (string, bool) {
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
			})
			r.Route("/devices", func(r chi.Router) {
				r.Post("/", deviceHandler.Create)
				r.Get("/", deviceHandler.List)
				r.Route("/{deviceID}", func(r chi.Router) {
					r.Patch("/", deviceHandler.Update)
					r.Delete("/", deviceHandler.Revoke)
					r.Post("/rotate", deviceHandler.Rotate)
				})
			})
		})
		r.Group(func(r chi.Router) {
			r.Use(deviceSvc.RequireDeviceOrOperator(authSvc.RequireAuth))
			r.Post("/events/{eventID}/uploads", uploadHandler.Initialize)
			r.Route("/uploads/{photoID}", func(r chi.Router) {
				r.Get("/", uploadHandler.Status)
				r.Post("/complete", uploadHandler.Complete)
				r.Post("/multipart/complete", uploadHandler.CompleteMultipart)
				r.Post("/multipart/abort", uploadHandler.AbortMultipart)
				r.Post("/parts", uploadHandler.Parts)
			})
		})
	})
	return r, pool, endpoint
}

func doReqHeaders(t *testing.T, h http.Handler, method, path, body, bearer string, headers map[string]string) *httptest.ResponseRecorder {
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
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestE2E_UploadSimpleFlow(t *testing.T) {
	h, pool := setupUploadsAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"ops@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Upload Event"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)

	// initialize a simple upload
	rec = doReqHeaders(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/uploads",
		`{"filename":"photo.jpg","contentType":"image/jpeg","size":12}`, access,
		map[string]string{"Idempotency-Key": "idem-1"})
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var initEnv struct {
		Data struct {
			PhotoID    string `json:"photoId"`
			UploadURL  string `json:"uploadUrl"`
			UploadKind string `json:"uploadKind"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &initEnv))
	require.Equal(t, "simple", initEnv.Data.UploadKind)
	require.NotEmpty(t, initEnv.Data.PhotoID)
	require.NotEmpty(t, initEnv.Data.UploadURL)

	// PUT bytes directly to MinIO via the presigned URL
	body := []byte("hello world!")
	req, err := http.NewRequest(http.MethodPut, initEnv.Data.UploadURL, bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// complete
	rec = doReqHeaders(t, h, http.MethodPost, "/api/v1/uploads/"+initEnv.Data.PhotoID+"/complete", "", access, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "PROCESSING")

	// a job row exists
	var jobs int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM jobs WHERE type = 'PROCESS_PHOTO'`).Scan(&jobs)
	require.NoError(t, err)
	require.Equal(t, 1, jobs)

	// status endpoint agrees
	rec = doReq(t, h, http.MethodGet, "/api/v1/uploads/"+initEnv.Data.PhotoID, "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "PROCESSING")

	// idempotent complete does not enqueue twice
	rec = doReqHeaders(t, h, http.MethodPost, "/api/v1/uploads/"+initEnv.Data.PhotoID+"/complete", "", access, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM jobs WHERE type = 'PROCESS_PHOTO'`).Scan(&jobs)
	require.NoError(t, err)
	require.Equal(t, 1, jobs)
}

func TestE2E_UploadTenantIsolation(t *testing.T) {
	h, _ := setupUploadsAPI(t)

	recA := doReq(t, h, http.MethodPost, "/api/v1/auth/signup", `{"email":"a@example.com","password":"password123"}`, "")
	accessA, _ := parseAuth(t, recA)
	recB := doReq(t, h, http.MethodPost, "/api/v1/auth/signup", `{"email":"b@example.com","password":"password123"}`, "")
	accessB, _ := parseAuth(t, recB)

	rec := doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"A Event"}`, accessA)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)

	// user B cannot initialize an upload for user A's event
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/uploads",
		`{"filename":"x.jpg","contentType":"image/jpeg","size":10}`, accessB)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")

	// user A initializes, user B cannot read its status
	rec = doReq(t, h, http.MethodPost, "/api/v1/events/"+eventID+"/uploads",
		`{"filename":"x.jpg","contentType":"image/jpeg","size":10}`, accessA)
	require.Equal(t, http.StatusCreated, rec.Code)
	var env struct {
		Data struct {
			PhotoID string `json:"photoId"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))

	rec = doReq(t, h, http.MethodGet, "/api/v1/uploads/"+env.Data.PhotoID, "", accessB)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "UPLOAD_NOT_FOUND")
}
