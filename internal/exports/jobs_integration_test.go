//go:build integration

package exports_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/exports"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startMinio(t *testing.T) string {
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

func createBucket(t *testing.T, endpoint, bucket string) {
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

func setupZipEnv(t *testing.T) (*pgxpool.Pool, *r2.S3Store, uuid.UUID, uuid.UUID, *photos.PostgresRepository) {
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
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE events (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		slug VARCHAR(255) NOT NULL,
		status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		photo_count BIGINT NOT NULL DEFAULT 0,
		guest_count BIGINT NOT NULL DEFAULT 0,
		expires_at TIMESTAMPTZ,
		expiry_warned_at TIMESTAMPTZ,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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

	userID := insertUser(t, pool, "zip@example.com")
	eventID := insertEvent(t, pool, userID, "zip", "zip")

	endpoint := startMinio(t)
	createBucket(t, endpoint, "cpd-exports")
	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-exports",
		Region:    "us-east-1",
		UseSSL:    false,
	})
	require.NoError(t, err)

	return pool, store, userID, eventID, photos.NewRepository(pool)
}

func seedReadyPhoto(t *testing.T, pool *pgxpool.Pool, store *r2.S3Store, eventID uuid.UUID, name, key, content string, createdAt time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO photos (event_id, storage_key, original_filename, mime_type, file_size, status, created_at, updated_at)
		 VALUES ($1, $2, $3, 'image/jpeg', $4, 'READY', $5, $5) RETURNING id`,
		eventID, key, name, int64(len(content)), createdAt).Scan(&id))
	require.NoError(t, store.Put(ctx, key, "image/jpeg", []byte(content)))
	return id
}

func zipLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestExportsZipWorker_Integration(t *testing.T) {
	ctx := context.Background()
	pool, store, userID, eventID, photosRepo := setupZipEnv(t)
	exportRepo := exports.NewRepository(pool)
	photosRepoForOwner := photosRepo

	t.Run("generate stores a download-ready archive", func(t *testing.T) {
		base := time.Now().UTC()
		seedReadyPhoto(t, pool, store, eventID, "IMG_1.jpg", "seed/1.jpg", "one", base)
		seedReadyPhoto(t, pool, store, eventID, "IMG_1.jpg", "seed/2.jpg", "two", base.Add(-time.Second))
		seedReadyPhoto(t, pool, store, eventID, "IMG_3.jpg", "seed/3.jpg", "three", base.Add(-2*time.Second))

		export, err := exportRepo.Create(ctx, eventID)
		require.NoError(t, err)

		payload, err := json.Marshal(exports.GeneratePayload{ExportID: export.ID, EventID: eventID})
		require.NoError(t, err)

		handler := exports.GenerateHandler(exportRepo, photosRepoForOwner.EventOwner, photosRepoForOwner, store, time.Hour, zipLogger())
		require.NoError(t, handler(ctx, payload))

		stored, err := exportRepo.GetByID(ctx, export.ID)
		require.NoError(t, err)
		require.Equal(t, exports.StatusReady, stored.Status)
		require.NotNil(t, stored.ObjectKey)
		require.Equal(t, r2.ExportKey(userID, eventID, export.ID), *stored.ObjectKey)

		size, err := store.Head(ctx, *stored.ObjectKey)
		require.NoError(t, err)
		require.NotNil(t, stored.FileSize)
		require.Equal(t, size, *stored.FileSize)

		reader, err := store.GetReader(ctx, *stored.ObjectKey)
		require.NoError(t, err)
		raw, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		require.NoError(t, err)
		require.Len(t, zr.File, 3)

		entries := map[string]string{}
		for _, f := range zr.File {
			rc, err := f.Open()
			require.NoError(t, err)
			body, err := io.ReadAll(rc)
			require.NoError(t, err)
			require.NoError(t, rc.Close())
			entries[f.Name] = string(body)
		}
		require.Equal(t, "one", entries["IMG_1.jpg"])
		require.Equal(t, "two", entries["IMG_1 (2).jpg"])
		require.Equal(t, "three", entries["IMG_3.jpg"])

		t.Run("cleanup expires and deletes the archive", func(t *testing.T) {
			mustExec(t, pool, `UPDATE exports SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, export.ID)

			cleanup := exports.CleanupHandler(exportRepo, store, zipLogger())
			require.NoError(t, cleanup(ctx, nil))

			stored, err := exportRepo.GetByID(ctx, export.ID)
			require.NoError(t, err)
			require.Equal(t, exports.StatusExpired, stored.Status)

			_, err = store.Head(ctx, *stored.ObjectKey)
			require.True(t, errors.Is(err, r2.ErrNotFound), "archive object should be deleted, got %v", err)
		})
	})

	t.Run("cleanup fails stale processing exports", func(t *testing.T) {
		export, err := exportRepo.Create(ctx, eventID)
		require.NoError(t, err)
		require.NoError(t, exportRepo.MarkProcessing(ctx, export.ID))
		mustExec(t, pool, `UPDATE exports SET updated_at = NOW() - INTERVAL '2 hours' WHERE id = $1`, export.ID)

		cleanup := exports.CleanupHandler(exportRepo, store, zipLogger())
		require.NoError(t, cleanup(ctx, nil))

		stored, err := exportRepo.GetByID(ctx, export.ID)
		require.NoError(t, err)
		require.Equal(t, exports.StatusFailed, stored.Status)
		require.NotNil(t, stored.ErrorMessage)
		require.Equal(t, "export timed out", *stored.ErrorMessage)
	})
}
