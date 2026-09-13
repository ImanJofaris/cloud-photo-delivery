//go:build integration

package analytics_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/analytics"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupAnalyticsDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
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
		photo_count BIGINT NOT NULL DEFAULT 0,
		deleted_at TIMESTAMPTZ,
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

	userA := insertUser(t, pool, "analytics-a@example.com")
	userB := insertUser(t, pool, "analytics-b@example.com")
	return pool, userA, userB
}

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func insertUser(t *testing.T, pool *pgxpool.Pool, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO users (email, password_hash) VALUES ($1, 'hash') RETURNING id`, email).Scan(&id))
	return id
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name, slug string, photos int64) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO events (user_id, name, slug, photo_count) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, name, slug, photos).Scan(&id))
	return id
}

func day(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	require.NoError(t, err)
	return parsed
}

func TestAnalyticsRepository_RecordViewUniqueVisitors(t *testing.T) {
	pool, userA, _ := setupAnalyticsDB(t)
	repo := analytics.NewRepository(pool)
	ctx := context.Background()
	eventID := insertEvent(t, pool, userA, "party", "party", 0)
	today := day(t, "2026-09-13")

	require.NoError(t, repo.RecordView(ctx, eventID, today, "hash-a", false))
	require.NoError(t, repo.RecordView(ctx, eventID, today, "hash-a", false))
	require.NoError(t, repo.RecordView(ctx, eventID, today, "hash-b", true))

	var views, visitors, downloads, scans int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT gallery_views, unique_visitors, downloads, qr_scans FROM event_analytics
		 WHERE event_id = $1 AND day = $2`, eventID, today).
		Scan(&views, &visitors, &downloads, &scans))
	require.EqualValues(t, 3, views)
	require.EqualValues(t, 2, visitors)
	require.EqualValues(t, 0, downloads)
	require.EqualValues(t, 1, scans)

	// The same visitor on another day is unique for that day.
	require.NoError(t, repo.RecordView(ctx, eventID, day(t, "2026-09-14"), "hash-a", false))
	summary, err := repo.EventSummary(ctx, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 4, summary.Totals.GalleryViews)
	require.EqualValues(t, 3, summary.Totals.UniqueVisitors)
	require.EqualValues(t, 1, summary.Totals.QRScans)
}

func TestAnalyticsRepository_RecordDownload(t *testing.T) {
	pool, userA, _ := setupAnalyticsDB(t)
	repo := analytics.NewRepository(pool)
	ctx := context.Background()
	eventID := insertEvent(t, pool, userA, "party", "party", 0)
	today := day(t, "2026-09-13")

	require.NoError(t, repo.RecordDownload(ctx, eventID, today))
	require.NoError(t, repo.RecordDownload(ctx, eventID, today))
	require.NoError(t, repo.RecordDownload(ctx, eventID, day(t, "2026-09-14")))

	summary, err := repo.EventSummary(ctx, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 3, summary.Totals.Downloads)
	require.EqualValues(t, 0, summary.Totals.GalleryViews)
}

func TestAnalyticsRepository_EventDailyWindow(t *testing.T) {
	pool, userA, _ := setupAnalyticsDB(t)
	repo := analytics.NewRepository(pool)
	ctx := context.Background()
	eventID := insertEvent(t, pool, userA, "party", "party", 12)

	require.NoError(t, repo.RecordView(ctx, eventID, day(t, "2026-09-10"), "hash-a", false))
	require.NoError(t, repo.RecordView(ctx, eventID, day(t, "2026-09-13"), "hash-a", true))

	daily, err := repo.EventDaily(ctx, eventID, day(t, "2026-09-12"))
	require.NoError(t, err)
	require.Len(t, daily, 1)
	require.Equal(t, "2026-09-13", daily[0].Day.Format("2006-01-02"))
	require.EqualValues(t, 1, daily[0].GalleryViews)
	require.EqualValues(t, 1, daily[0].QRScans)

	summary, err := repo.EventSummary(ctx, eventID)
	require.NoError(t, err)
	require.EqualValues(t, 12, summary.PhotoCount)
	require.EqualValues(t, 2, summary.Totals.GalleryViews, "totals include days outside the window")
}

func TestAnalyticsRepository_AccountScoping(t *testing.T) {
	pool, userA, userB := setupAnalyticsDB(t)
	repo := analytics.NewRepository(pool)
	ctx := context.Background()

	eventA1 := insertEvent(t, pool, userA, "a1", "a1", 10)
	eventA2 := insertEvent(t, pool, userA, "a2", "a2", 5)
	eventB := insertEvent(t, pool, userB, "b1", "b1", 7)

	require.NoError(t, repo.RecordView(ctx, eventA1, day(t, "2026-09-13"), "hash-a", true))
	require.NoError(t, repo.RecordView(ctx, eventA2, day(t, "2026-09-13"), "hash-b", false))
	require.NoError(t, repo.RecordView(ctx, eventB, day(t, "2026-09-13"), "hash-c", false))

	// A third event of user A, soft-deleted, must be excluded.
	eventA3 := insertEvent(t, pool, userA, "a3", "a3", 3)
	require.NoError(t, repo.RecordView(ctx, eventA3, day(t, "2026-09-13"), "hash-d", false))
	mustExec(t, pool, `UPDATE events SET deleted_at = NOW() WHERE id = $1`, eventA3)

	summary, err := repo.AccountSummary(ctx, userA)
	require.NoError(t, err)
	require.EqualValues(t, 2, summary.EventCount)
	require.EqualValues(t, 15, summary.PhotoCount)
	require.EqualValues(t, 2, summary.Totals.GalleryViews)
	require.EqualValues(t, 1, summary.Totals.QRScans)

	daily, err := repo.AccountDaily(ctx, userA, day(t, "2026-09-13"))
	require.NoError(t, err)
	require.Len(t, daily, 1)
	require.EqualValues(t, 2, daily[0].GalleryViews)
	require.EqualValues(t, 2, daily[0].UniqueVisitors)

	other, err := repo.AccountSummary(ctx, userB)
	require.NoError(t, err)
	require.EqualValues(t, 1, other.Totals.GalleryViews, "other tenants are never included")
}
