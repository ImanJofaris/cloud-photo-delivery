//go:build integration

package photos_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/uploads"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
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
	userA := insertUser(t, pool, "a@example.com")
	userB := insertUser(t, pool, "b@example.com")
	return pool, userA, userB
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`, email, "hash").Scan(&id)
	require.NoError(t, err)
	return id
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (user_id, name, slug) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, name+"-slug").Scan(&id)
	require.NoError(t, err)
	return id
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func createUploadingPhoto(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID, size int64) *photos.Photo {
	t.Helper()
	repo := photos.NewRepository(pool)
	p, err := repo.Create(context.Background(), photos.CreateInput{
		EventID:          eventID,
		StorageKey:       "tenant/x/events/y/originals/z/a.jpg",
		OriginalFilename: "a.jpg",
		MimeType:         "image/jpeg",
		FileSize:         size,
		Status:           photos.StatusUploading,
		UploadKind:       photos.KindSimple,
	})
	require.NoError(t, err)
	return p
}

func TestPhotosRepository_CreateAndGet(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")

	p := createUploadingPhoto(t, pool, eventID, 2048)
	repo := photos.NewRepository(pool)

	got, err := repo.GetByID(context.Background(), p.ID)
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ID)
	require.Equal(t, photos.StatusUploading, got.Status)
	require.EqualValues(t, 2048, got.FileSize)
}

func TestPhotosRepository_MarkProcessingIncrementsCountersOnce(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 4096)
	repo := photos.NewRepository(pool)

	updated, transitioned, err := repo.MarkProcessing(context.Background(), p.ID, 4096)
	require.NoError(t, err)
	require.True(t, transitioned)
	require.Equal(t, photos.StatusProcessing, updated.Status)

	_, transitioned, err = repo.MarkProcessing(context.Background(), p.ID, 4096)
	require.NoError(t, err)
	require.False(t, transitioned, "second transition must be a no-op")

	var photoCount, storageBytes int64
	err = pool.QueryRow(context.Background(),
		`SELECT photo_count, storage_bytes FROM events WHERE id = $1`, eventID).Scan(&photoCount, &storageBytes)
	require.NoError(t, err)
	require.EqualValues(t, 1, photoCount)
	require.EqualValues(t, 4096, storageBytes)

	var userStorage int64
	err = pool.QueryRow(context.Background(),
		`SELECT storage_bytes FROM users WHERE id = $1`, userA).Scan(&userStorage)
	require.NoError(t, err)
	require.EqualValues(t, 4096, userStorage, "user storage increments in the same transaction")
}

func TestPhotosRepository_CountByEventExcludesFailed(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	first := createUploadingPhoto(t, pool, eventID, 100)
	second := createUploadingPhoto(t, pool, eventID, 100)
	require.NoError(t, repo.MarkFailed(ctx, second.ID, "bad image"))

	count, err := repo.CountByEvent(ctx, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count, "FAILED photos do not count against quota")

	_, _, err = repo.MarkProcessing(ctx, first.ID, 0)
	require.NoError(t, err)
	count, err = repo.CountByEvent(ctx, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func TestPhotosRepository_SavePartETagUpsert(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 100)
	repo := photos.NewRepository(pool)

	require.NoError(t, repo.SavePartETag(context.Background(), p.ID, 1, "etag-1"))
	require.NoError(t, repo.SavePartETag(context.Background(), p.ID, 1, "etag-1b"))

	var etag string
	err := pool.QueryRow(context.Background(),
		`SELECT etag FROM upload_parts WHERE photo_id = $1 AND part_number = 1`, p.ID).Scan(&etag)
	require.NoError(t, err)
	require.Equal(t, "etag-1b", etag)
}

func TestPhotosRepository_CascadeDeleteOnEventDelete(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 100)
	repo := photos.NewRepository(pool)
	require.NoError(t, repo.SavePartETag(context.Background(), p.ID, 1, "etag-1"))

	_, err := pool.Exec(context.Background(), `DELETE FROM events WHERE id = $1`, eventID)
	require.NoError(t, err)

	_, err = repo.GetByID(context.Background(), p.ID)
	require.ErrorIs(t, err, photos.ErrNotFound)

	var parts int
	err = pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM upload_parts WHERE photo_id = $1`, p.ID).Scan(&parts)
	require.NoError(t, err)
	require.Zero(t, parts)
}

func TestUploadsRepository_TenantIsolationOnEventOwnership(t *testing.T) {
	pool, userA, userB := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := uploads.NewRepository(pool)

	owned, err := repo.EventOwnedBy(context.Background(), userA, eventID)
	require.NoError(t, err)
	require.True(t, owned)

	owned, err = repo.EventOwnedBy(context.Background(), userB, eventID)
	require.NoError(t, err)
	require.False(t, owned, "user B must not own user A's event")
}

func TestUploadsRepository_IdempotencyUniqueConstraint(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 100)
	repo := uploads.NewRepository(pool)
	ctx := context.Background()

	rec := uploads.IdempotencyRecord{UserID: userA, PhotoID: p.ID, RequestHash: "h1"}
	require.NoError(t, repo.SaveIdempotency(ctx, "key-1", rec))
	require.NoError(t, repo.SaveIdempotency(ctx, "key-1", rec))

	got, err := repo.LookupIdempotency(ctx, "key-1")
	require.NoError(t, err)
	require.Equal(t, p.ID, got.PhotoID)
	require.Equal(t, "h1", got.RequestHash)

	_, err = repo.LookupIdempotency(ctx, "missing")
	require.ErrorIs(t, err, uploads.ErrNotFound)
}

func TestPostgresQueue_EnqueuesProcessPhoto(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 100)

	q := uploads.NewPostgresQueue(pool)
	require.NoError(t, q.EnqueueProcessPhoto(context.Background(), p.ID, eventID))

	var jobType, status string
	err := pool.QueryRow(context.Background(), `SELECT type, status FROM jobs LIMIT 1`).Scan(&jobType, &status)
	require.NoError(t, err)
	require.Equal(t, "PROCESS_PHOTO", jobType)
	require.Equal(t, "pending", status)
}

func setCreatedAt(t *testing.T, pool *pgxpool.Pool, photoID uuid.UUID, at time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `UPDATE photos SET created_at = $2 WHERE id = $1`, photoID, at)
	require.NoError(t, err)
}

func eventCounters(t *testing.T, pool *pgxpool.Pool, eventID uuid.UUID) (int64, int64) {
	t.Helper()
	var photoCount, storageBytes int64
	err := pool.QueryRow(context.Background(),
		`SELECT photo_count, storage_bytes FROM events WHERE id = $1`, eventID).Scan(&photoCount, &storageBytes)
	require.NoError(t, err)
	return photoCount, storageBytes
}

func TestPhotosRepository_ListByEventPaginationAndFilter(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	p1 := createUploadingPhoto(t, pool, eventID, 100)
	p2 := createUploadingPhoto(t, pool, eventID, 100)
	p3 := createUploadingPhoto(t, pool, eventID, 100)
	base := time.Now().UTC()
	setCreatedAt(t, pool, p1.ID, base.Add(-2*time.Minute))
	setCreatedAt(t, pool, p2.ID, base.Add(-1*time.Minute))
	setCreatedAt(t, pool, p3.ID, base)

	page1, err := repo.ListByEvent(ctx, photos.ListInput{EventID: eventID, UserID: userA, Limit: 2})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, p3.ID, page1[0].ID)
	require.Equal(t, p2.ID, page1[1].ID)

	last := page1[len(page1)-1]
	page2, err := repo.ListByEvent(ctx, photos.ListInput{
		EventID: eventID,
		UserID:  userA,
		Cursor:  &photos.Cursor{CreatedAt: last.CreatedAt, ID: last.ID},
		Limit:   2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 1)
	require.Equal(t, p1.ID, page2[0].ID)

	filtered, err := repo.ListByEvent(ctx, photos.ListInput{
		EventID: eventID,
		UserID:  userA,
		Status:  photos.StatusReady,
		Limit:   10,
	})
	require.NoError(t, err)
	require.Empty(t, filtered)
}

func TestPhotosRepository_ListByEventTenantIsolation(t *testing.T) {
	pool, userA, userB := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	createUploadingPhoto(t, pool, eventID, 100)
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	mine, err := repo.ListByEvent(ctx, photos.ListInput{EventID: eventID, UserID: userA, Limit: 10})
	require.NoError(t, err)
	require.Len(t, mine, 1)

	theirs, err := repo.ListByEvent(ctx, photos.ListInput{EventID: eventID, UserID: userB, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, theirs, "user B must not list user A's photos")

	_, err = pool.Exec(ctx, `UPDATE events SET deleted_at = NOW() WHERE id = $1`, eventID)
	require.NoError(t, err)
	afterDelete, err := repo.ListByEvent(ctx, photos.ListInput{EventID: eventID, UserID: userA, Limit: 10})
	require.NoError(t, err)
	require.Empty(t, afterDelete, "soft-deleted events are not listable")
}

func TestPhotosRepository_DeleteOwnedAdjustsCountersOnce(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	p := createUploadingPhoto(t, pool, eventID, 4096)
	require.NoError(t, repo.SavePartETag(ctx, p.ID, 1, "etag-1"))
	_, transitioned, err := repo.MarkProcessing(ctx, p.ID, 4096)
	require.NoError(t, err)
	require.True(t, transitioned)
	require.NoError(t, repo.MarkReady(ctx, p.ID, 100, 50, photos.Derivatives{
		Thumbnail: photos.Derivative{Key: "thumb.webp"},
		Medium:    photos.Derivative{Key: "medium.webp"},
		Optimized: photos.Derivative{Key: "large.webp"},
	}))

	deleted, err := repo.DeleteOwned(ctx, p.ID, userA)
	require.NoError(t, err)
	require.True(t, deleted.Counted)
	require.Equal(t, eventID, deleted.EventID)
	require.ElementsMatch(t, []string{p.StorageKey, "thumb.webp", "medium.webp", "large.webp"}, deleted.Keys)

	photoCount, storageBytes := eventCounters(t, pool, eventID)
	require.Zero(t, photoCount)
	require.Zero(t, storageBytes)

	var userStorage int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE id = $1`, userA).Scan(&userStorage))
	require.Zero(t, userStorage, "user storage decrements in the same transaction")

	_, err = repo.GetByID(ctx, p.ID)
	require.ErrorIs(t, err, photos.ErrNotFound)

	var parts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM upload_parts WHERE photo_id = $1`, p.ID).Scan(&parts))
	require.Zero(t, parts)

	_, err = repo.DeleteOwned(ctx, p.ID, userA)
	require.ErrorIs(t, err, photos.ErrNotFound)
}

func TestPhotosRepository_DeleteOwnedUploadingNotCounted(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	p := createUploadingPhoto(t, pool, eventID, 2048)
	deleted, err := repo.DeleteOwned(ctx, p.ID, userA)
	require.NoError(t, err)
	require.False(t, deleted.Counted, "uploading photos never incremented counters")

	photoCount, storageBytes := eventCounters(t, pool, eventID)
	require.Zero(t, photoCount)
	require.Zero(t, storageBytes)
}

func TestPhotosRepository_DeleteOwnedTenantIsolation(t *testing.T) {
	pool, userA, userB := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	p := createUploadingPhoto(t, pool, eventID, 2048)
	_, err := repo.DeleteOwned(ctx, p.ID, userB)
	require.ErrorIs(t, err, photos.ErrNotFound)

	_, err = repo.GetByID(ctx, p.ID)
	require.NoError(t, err, "user B must not delete user A's photo")
}

func TestPhotosRepository_EventOwnedBy(t *testing.T) {
	pool, userA, userB := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	repo := photos.NewRepository(pool)
	ctx := context.Background()

	owned, err := repo.EventOwnedBy(ctx, userA, eventID)
	require.NoError(t, err)
	require.True(t, owned)

	owned, err = repo.EventOwnedBy(ctx, userB, eventID)
	require.NoError(t, err)
	require.False(t, owned)
}

func TestPhotosQueue_EnqueuesObjectCleanup(t *testing.T) {
	pool, userA, _ := setupDB(t)
	eventID := insertEvent(t, pool, userA, "wedding")
	p := createUploadingPhoto(t, pool, eventID, 100)

	q := photos.NewPostgresQueue(pool)
	payload := photos.CleanupPayload{PhotoID: p.ID, EventID: eventID, Keys: []string{"orig/a.jpg", "thumb/a.webp"}}
	require.NoError(t, q.EnqueueObjectCleanup(context.Background(), payload))

	var jobType string
	var raw []byte
	err := pool.QueryRow(context.Background(), `SELECT type, payload FROM jobs LIMIT 1`).Scan(&jobType, &raw)
	require.NoError(t, err)
	require.Equal(t, "object.cleanup", jobType)

	var got photos.CleanupPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, payload.PhotoID, got.PhotoID)
	require.ElementsMatch(t, payload.Keys, got.Keys)
}
