//go:build integration

package gallery_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/gallery"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) (*pgxpool.Pool, uuid.UUID) {
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
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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

	var userID uuid.UUID
	err = pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('a@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	return pool, userID
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func seedEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, slug, visibility string, passwordChangedAt *time.Time) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var eventID uuid.UUID
	_, err := pool.Exec(ctx,
		`INSERT INTO events (user_id, name, slug) VALUES ($1, $2, $3)`, userID, "Event "+slug, slug)
	require.NoError(t, err)
	err = pool.QueryRow(ctx, `SELECT id FROM events WHERE slug = $1`, slug).Scan(&eventID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO event_settings (event_id, visibility, password_hash, password_changed_at) VALUES ($1, $2, $3, $4)`,
		eventID, visibility, "hash", passwordChangedAt)
	require.NoError(t, err)
	return eventID
}

func insertPhoto(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO photos (event_id, storage_key, mime_type, file_size, status, thumbnail_key, medium_key, optimized_key, width, height, created_at)
		 VALUES ($1, $2, 'image/jpeg', 10, $3, 't', 'm', 'o', 100, 100, $4) RETURNING id`,
		eventID, "orig/"+uuid.NewString(), status, createdAt).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestRepository_EventBySlug(t *testing.T) {
	pool, userID := setupDB(t)
	repo := gallery.NewRepository(pool)
	ctx := context.Background()

	eventID := seedEvent(t, pool, userID, "wedding", "public", nil)
	e, s, err := repo.EventBySlug(ctx, "wedding")
	require.NoError(t, err)
	require.Equal(t, eventID, e.ID)
	require.Equal(t, "public", string(s.Visibility))

	_, _, err = repo.EventBySlug(ctx, "missing")
	require.ErrorIs(t, err, gallery.ErrNotFound)
}

func TestRepository_EventBySlug_ExcludesDeletedAndExpired(t *testing.T) {
	pool, userID := setupDB(t)
	repo := gallery.NewRepository(pool)
	ctx := context.Background()

	seedEvent(t, pool, userID, "deleted", "public", nil)
	_, err := pool.Exec(ctx, `UPDATE events SET deleted_at = NOW() WHERE slug = 'deleted'`)
	require.NoError(t, err)
	_, _, err = repo.EventBySlug(ctx, "deleted")
	require.ErrorIs(t, err, gallery.ErrNotFound)

	seedEvent(t, pool, userID, "expired", "public", nil)
	_, err = pool.Exec(ctx, `UPDATE events SET expires_at = NOW() - INTERVAL '1 hour' WHERE slug = 'expired'`)
	require.NoError(t, err)
	_, _, err = repo.EventBySlug(ctx, "expired")
	require.ErrorIs(t, err, gallery.ErrNotFound)
}

func TestRepository_ListReadyPhotos_KeysetStable(t *testing.T) {
	pool, userID := setupDB(t)
	repo := gallery.NewRepository(pool)
	ctx := context.Background()

	eventID := seedEvent(t, pool, userID, "wedding", "public", nil)
	base := time.Now().UTC().Truncate(time.Microsecond)

	// Same timestamp to prove ID tiebreak is stable; plus non-READY rows.
	for i := 0; i < 3; i++ {
		insertPhoto(t, pool, eventID, "READY", base)
	}
	insertPhoto(t, pool, eventID, "PROCESSING", base)
	insertPhoto(t, pool, eventID, "FAILED", base)

	seen := map[uuid.UUID]bool{}
	var cursor *gallery.Cursor
	total := 0
	for {
		page, err := repo.ListReadyPhotos(ctx, eventID, cursor, 2)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, p := range page {
			require.False(t, seen[p.ID], "duplicate photo across pages")
			seen[p.ID] = true
			total++
		}
		last := page[len(page)-1]
		cursor = &gallery.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
		if len(page) < 2 {
			break
		}
	}
	require.Equal(t, 3, total)
}

func TestRepository_ReadyPhoto_ScopesEventAndStatus(t *testing.T) {
	pool, userID := setupDB(t)
	repo := gallery.NewRepository(pool)
	ctx := context.Background()

	eventA := seedEvent(t, pool, userID, "event-a", "public", nil)
	eventB := seedEvent(t, pool, userID, "event-b", "public", nil)
	readyID := insertPhoto(t, pool, eventA, "READY", time.Now())
	processingID := insertPhoto(t, pool, eventA, "PROCESSING", time.Now())

	_, err := repo.ReadyPhoto(ctx, eventA, readyID)
	require.NoError(t, err)

	_, err = repo.ReadyPhoto(ctx, eventB, readyID)
	require.ErrorIs(t, err, gallery.ErrNotFound)

	_, err = repo.ReadyPhoto(ctx, eventA, processingID)
	require.ErrorIs(t, err, gallery.ErrNotFound)
}

func TestRepository_GetSettingsByEventID(t *testing.T) {
	pool, userID := setupDB(t)
	repo := gallery.NewRepository(pool)
	ctx := context.Background()

	eventID := seedEvent(t, pool, userID, "wedding", "password", nil)
	s, err := repo.GetSettingsByEventID(ctx, eventID)
	require.NoError(t, err)
	require.Equal(t, "password", string(s.Visibility))
	require.Equal(t, "hash", s.PasswordHash)
}

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

func TestSignedURL_RetrievesObjectFromMinIO(t *testing.T) {
	ctx := context.Background()
	endpoint := startMinio(t)

	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-photos",
		Region:    "us-east-1",
	})
	require.NoError(t, err)
	createBucket(t, endpoint)

	key := "tenant/x/events/y/optimized/z.webp"
	require.NoError(t, store.Put(ctx, key, "image/webp", []byte("derivative-bytes")))

	opt := key
	p := &photos.Photo{ID: uuid.New(), StorageKey: "orig/a.jpg", OptimizedKey: &opt}
	gen := photos.NewSignedURLGenerator(store, time.Minute)
	res, err := gen.URL(ctx, p, "large")
	require.NoError(t, err)
	require.Equal(t, 60, res.ExpiresIn)

	resp, err := http.Get(res.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func createBucket(t *testing.T, endpoint string) {
	t.Helper()
	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("us-east-1"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("minioadmin", "minioadmin", "")),
	)
	require.NoError(t, err)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})
	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String("cpd-photos")})
	require.NoError(t, err)
}
