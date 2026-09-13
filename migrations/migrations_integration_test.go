//go:build integration

package migrations_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestMigrations_DevicesUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	devicesSQL, err := os.ReadFile("0007_devices.sql")
	require.NoError(t, err)
	up := extractSection(string(devicesSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'devices')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "devices table should exist after up")

	for _, index := range []string{"idx_devices_user", "idx_devices_prefix"} {
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`, index).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "%s should exist after up", index)
	}

	down := extractSection(string(devicesSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'devices')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "devices table should be dropped by down")
}

func TestMigrations_BrandingUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	brandingSQL, err := os.ReadFile("0008_branding.sql")
	require.NoError(t, err)
	up := extractSection(string(brandingSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tenant_branding')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "tenant_branding table should exist after up")

	down := extractSection(string(brandingSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'tenant_branding')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "tenant_branding table should be dropped by down")
}

func TestMigrations_BillingUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql", "0008_branding.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('bill@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO events (user_id, name, slug, storage_bytes) VALUES ($1, 'wedding', 'wedding', 500)`, userID)
	require.NoError(t, err)

	billingSQL, err := os.ReadFile("0009_billing.sql")
	require.NoError(t, err)
	up := extractSection(string(billingSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	for _, table := range []string{"plans", "subscriptions", "invoices", "billing_webhook_events"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after up", table)
	}

	var storageBytes int64
	err = pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE id = $1`, userID).Scan(&storageBytes)
	require.NoError(t, err)
	require.EqualValues(t, 500, storageBytes, "existing event storage is backfilled")

	var planCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM plans`).Scan(&planCount)
	require.NoError(t, err)
	require.Equal(t, 4, planCount, "four plans are seeded")

	down := extractSection(string(billingSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	for _, table := range []string{"plans", "subscriptions", "invoices", "billing_webhook_events"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should be dropped by down", table)
	}
	var columnExists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'storage_bytes')`).
		Scan(&columnExists)
	require.NoError(t, err)
	require.False(t, columnExists, "users.storage_bytes should be dropped by down")
}

func TestMigrations_LifecycleUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql",
		"0008_branding.sql", "0009_billing.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('life@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)

	lifecycleSQL, err := os.ReadFile("0010_lifecycle.sql")
	require.NoError(t, err)
	up := extractSection(string(lifecycleSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'events' AND column_name = 'expiry_warned_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "events.expiry_warned_at should exist after up")

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_events_expires_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "idx_events_expires_at should exist after up")

	_, err = pool.Exec(ctx,
		`INSERT INTO events (user_id, name, slug, status) VALUES ($1, 'expired', 'expired', 'expired')`, userID)
	require.NoError(t, err, "expired status must be accepted after up")

	down := extractSection(string(lifecycleSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'events' AND column_name = 'expiry_warned_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "events.expiry_warned_at should be dropped by down")

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_events_expires_at')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "idx_events_expires_at should be dropped by down")

	var status string
	err = pool.QueryRow(ctx, `SELECT status FROM events WHERE slug = 'expired'`).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "archived", status, "expired rows are archived by down")

	_, err = pool.Exec(ctx,
		`INSERT INTO events (user_id, name, slug, status) VALUES ($1, 'expired again', 'expired-again', 'expired')`, userID)
	require.Error(t, err, "expired status must be rejected after down")
}

func TestMigrations_ExportsUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql",
		"0008_branding.sql", "0009_billing.sql", "0010_lifecycle.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('export@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	var eventID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO events (user_id, name, slug) VALUES ($1, 'export', 'export') RETURNING id`, userID).Scan(&eventID)
	require.NoError(t, err)

	exportsSQL, err := os.ReadFile("0011_exports.sql")
	require.NoError(t, err)
	up := extractSection(string(exportsSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'exports')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "exports table should exist after up")

	for _, index := range []string{"idx_exports_event_created", "idx_exports_status_expires", "idx_exports_active_event"} {
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`, index).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "%s should exist after up", index)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO exports (id, event_id, status) VALUES (gen_random_uuid(), $1, 'pending')`, eventID)
	require.NoError(t, err, "pending status must be accepted after up")

	_, err = pool.Exec(ctx,
		`INSERT INTO exports (id, event_id, status) VALUES (gen_random_uuid(), $1, 'bogus')`, eventID)
	require.Error(t, err, "unknown status must be rejected after up")

	down := extractSection(string(exportsSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'exports')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "exports table should be dropped by down")
}

func TestMigrations_AnalyticsUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql",
		"0008_branding.sql", "0009_billing.sql", "0010_lifecycle.sql", "0011_exports.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('analytics@example.com', 'hash') RETURNING id`).Scan(&userID)
	require.NoError(t, err)
	var eventID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO events (user_id, name, slug) VALUES ($1, 'analytics', 'analytics') RETURNING id`, userID).Scan(&eventID)
	require.NoError(t, err)

	analyticsSQL, err := os.ReadFile("0012_analytics.sql")
	require.NoError(t, err)
	up := extractSection(string(analyticsSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	for _, table := range []string{"event_analytics", "event_visitors"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		require.NoError(t, err)
		require.True(t, exists, "table %s should exist after up", table)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO event_analytics (event_id, day, gallery_views) VALUES ($1, '2026-09-13', 1)`, eventID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO event_visitors (event_id, day, visitor_hash) VALUES ($1, '2026-09-13', 'hash')`, eventID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO event_visitors (event_id, day, visitor_hash) VALUES ($1, '2026-09-13', 'hash')`, eventID)
	require.Error(t, err, "a visitor is unique per event and day")

	down := extractSection(string(analyticsSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	for _, table := range []string{"event_analytics", "event_visitors"} {
		var exists bool
		err = pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, table).Scan(&exists)
		require.NoError(t, err)
		require.False(t, exists, "table %s should be dropped by down", table)
	}
}

func TestMigrations_AdminUpDownRoundTrip(t *testing.T) {
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

	for _, file := range []string{
		"0001_init.sql", "0002_auth.sql", "0003_events.sql",
		"0004_photos.sql", "0005_jobs.sql", "0006_gallery.sql", "0007_devices.sql",
		"0008_branding.sql", "0009_billing.sql", "0010_lifecycle.sql", "0011_exports.sql",
		"0012_analytics.sql",
	} {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, extractSection(string(sql), "-- +goose Up", "-- +goose Down"))
		require.NoError(t, err)
	}

	adminSQL, err := os.ReadFile("0013_admin.sql")
	require.NoError(t, err)
	up := extractSection(string(adminSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)
	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'is_admin')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "users.is_admin should exist after up")

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_users_is_admin')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists, "idx_users_is_admin should exist after up")

	var isAdmin bool
	err = pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('admin@example.com', 'hash')
		 RETURNING is_admin`).Scan(&isAdmin)
	require.NoError(t, err)
	require.False(t, isAdmin, "is_admin defaults to false")

	_, err = pool.Exec(ctx, `UPDATE users SET is_admin = TRUE WHERE email = 'admin@example.com'`)
	require.NoError(t, err)
	err = pool.QueryRow(ctx,
		`SELECT is_admin FROM users WHERE email = 'admin@example.com'`).Scan(&isAdmin)
	require.NoError(t, err)
	require.True(t, isAdmin)

	down := extractSection(string(adminSQL), "-- +goose Down", "__never__")
	require.NotEmpty(t, down)
	_, err = pool.Exec(ctx, down)
	require.NoError(t, err)

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'is_admin')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "users.is_admin should be dropped by down")

	err = pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'idx_users_is_admin')`,
	).Scan(&exists)
	require.NoError(t, err)
	require.False(t, exists, "idx_users_is_admin should be dropped by down")
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
