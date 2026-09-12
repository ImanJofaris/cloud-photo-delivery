//go:build integration

package migrations_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrations_UpSQLApplies(t *testing.T) {
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

	upSQL, err := os.ReadFile("0001_init.sql")
	require.NoError(t, err)

	// Extract the Up section between "-- +goose Up" and "-- +goose Down".
	up := extractSection(string(upSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)

	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'schema_migrations_baseline')",
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}

func TestMigrations_EventsUpDownRoundTrip(t *testing.T) {
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

	initSQL, err := os.ReadFile("0001_init.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, extractSection(string(initSQL), "-- +goose Up", "-- +goose Down"))
	require.NoError(t, err)

	authSQL, err := os.ReadFile("0002_auth.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, extractSection(string(authSQL), "-- +goose Up", "-- +goose Down"))
	require.NoError(t, err)

	eventsSQL, err := os.ReadFile("0003_events.sql")
	require.NoError(t, err)
	up := extractSection(string(eventsSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	for _, table := range []string{"events", "event_settings"} {
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after up", table)
	}

	down := extractSection(string(eventsSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	for _, table := range []string{"events", "event_settings"} {
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should be dropped by down", table)
	}
}

func TestMigrations_PhotosUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{"0001_init.sql", "0002_auth.sql", "0003_events.sql"} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	photosSQL, err := os.ReadFile("0004_photos.sql")
	require.NoError(t, err)
	up := extractSection(string(photosSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	for _, table := range []string{"photos", "upload_idempotency", "upload_parts", "jobs"} {
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after up", table)
	}

	down := extractSection(string(photosSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	for _, table := range []string{"photos", "upload_idempotency", "upload_parts", "jobs"} {
		var exists bool
		err = pool.QueryRow(ctx,
			"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)", table).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should be dropped by down", table)
	}
}

func TestMigrations_JobsUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{"0001_init.sql", "0002_auth.sql", "0003_events.sql", "0004_photos.sql"} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	jobsSQL, err := os.ReadFile("0005_jobs.sql")
	require.NoError(t, err)
	up := extractSection(string(jobsSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	for _, column := range []string{"max_attempts", "locked_at", "locked_by"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'jobs' AND column_name = $1)`,
			column).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "jobs.%s should exist after up", column)
	}

	down := extractSection(string(jobsSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	for _, column := range []string{"max_attempts", "locked_at", "locked_by"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'jobs' AND column_name = $1)`,
			column).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "jobs.%s should be dropped by down", column)
	}
}

func TestMigrations_GalleryUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{"0001_init.sql", "0002_auth.sql", "0003_events.sql", "0004_photos.sql", "0005_jobs.sql"} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	gallerySQL, err := os.ReadFile("0006_gallery.sql")
	require.NoError(t, err)
	up := extractSection(string(gallerySQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'event_settings' AND column_name = 'password_changed_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "event_settings.password_changed_at should exist after up")

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_photos_event_status_created')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "idx_photos_event_status_created should exist after up")

	down := extractSection(string(gallerySQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'event_settings' AND column_name = 'password_changed_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "event_settings.password_changed_at should be dropped by down")
}

func extractSection(s, start, end string) string {
	i := indexOf(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := indexOf(s[i:], end)
	if j < 0 {
		return s[i:]
	}
	return s[i : i+j]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
