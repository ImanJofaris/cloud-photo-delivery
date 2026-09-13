//go:build integration

package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
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
		email_verified_at TIMESTAMPTZ,
		failed_login_count INT NOT NULL DEFAULT 0,
		locked_until TIMESTAMPTZ,
		storage_bytes BIGINT NOT NULL DEFAULT 0,
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
		expiry_warned_at TIMESTAMPTZ,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT events_status_check CHECK (status IN ('upcoming','active','completed','archived','expired')),
		CONSTRAINT events_slug_unique UNIQUE (user_id, slug)
	)`)
	mustExec(t, pool, `CREATE TABLE photos (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		storage_key TEXT NOT NULL,
		thumbnail_key TEXT,
		optimized_key TEXT,
		medium_key TEXT,
		status VARCHAR(30) NOT NULL DEFAULT 'READY'
	)`)
	mustExec(t, pool, `CREATE TABLE exports (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		status VARCHAR(20) NOT NULL,
		object_key TEXT,
		file_size BIGINT,
		error_message TEXT,
		expires_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE event_settings (
		event_id UUID PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
		visibility VARCHAR(30) NOT NULL DEFAULT 'public',
		password_hash TEXT,
		allow_download BOOLEAN NOT NULL DEFAULT TRUE,
		allow_original_download BOOLEAN NOT NULL DEFAULT FALSE,
		watermark_enabled BOOLEAN NOT NULL DEFAULT FALSE,
		password_changed_at TIMESTAMPTZ,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT event_settings_visibility_check CHECK (visibility IN ('public','password','private'))
	)`)
	mustExec(t, pool, `CREATE INDEX idx_events_user_status ON events(user_id, status)`)
	mustExec(t, pool, `CREATE INDEX idx_events_user_created ON events(user_id, created_at DESC, id DESC)`)

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

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
}

func defaultSettings() events.Settings {
	return events.Settings{
		Visibility:    events.VisibilityPublic,
		AllowDownload: true,
	}
}

func createInput(userID uuid.UUID, name, slug string) events.CreateInput {
	return events.CreateInput{
		UserID:   userID,
		Name:     name,
		Slug:     slug,
		Status:   events.StatusUpcoming,
		Settings: defaultSettings(),
	}
}

func TestRepository_CreateAndGet(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, s, err := repo.Create(ctx, createInput(userA, "Summer Party", "summer-party"))
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, e.ID)
	require.Equal(t, "summer-party", e.Slug)
	require.Equal(t, events.StatusUpcoming, e.Status)
	require.Equal(t, e.ID, s.EventID)
	require.Equal(t, events.VisibilityPublic, s.Visibility)
	require.True(t, s.AllowDownload)

	got, err := repo.GetByID(ctx, userA, e.ID)
	require.NoError(t, err)
	require.Equal(t, e.ID, got.ID)
	require.Equal(t, "Summer Party", got.Name)
}

func TestRepository_UniqueSlugPerTenant(t *testing.T) {
	pool, userA, userB := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	_, _, err := repo.Create(ctx, createInput(userA, "Party", "party"))
	require.NoError(t, err)

	_, _, err = repo.Create(ctx, createInput(userA, "Party Again", "party"))
	require.Error(t, err)

	_, _, err = repo.Create(ctx, createInput(userB, "Party", "party"))
	require.NoError(t, err, "same slug must be allowed for a different tenant")
}

func TestRepository_TenantIsolation(t *testing.T) {
	pool, userA, userB := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Secret", "secret"))
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, userB, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound)

	_, err = repo.Update(ctx, userB, e.ID, events.UpdateInput{Name: strPtr("Hacked")})
	require.ErrorIs(t, err, events.ErrNotFound)

	err = repo.SoftDelete(ctx, userB, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound)

	// Owner still has access.
	_, err = repo.GetByID(ctx, userA, e.ID)
	require.NoError(t, err)
}

func TestRepository_SoftDeleteExcluded(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Temp", "temp"))
	require.NoError(t, err)

	require.NoError(t, repo.SoftDelete(ctx, userA, e.ID))

	_, err = repo.GetByID(ctx, userA, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound)

	items, err := repo.List(ctx, userA, events.ListFilter{})
	require.NoError(t, err)
	require.Empty(t, items)

	err = repo.SoftDelete(ctx, userA, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound, "second delete is a no-op not-found")
}

func TestRepository_Update(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Old", "old"))
	require.NoError(t, err)

	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	updated, err := repo.Update(ctx, userA, e.ID, events.UpdateInput{
		Name:      strPtr("New"),
		EventDate: &date,
	})
	require.NoError(t, err)
	require.Equal(t, "New", updated.Name)
	require.NotNil(t, updated.EventDate)
	require.Equal(t, "2026-06-01", updated.EventDate.Format("2006-01-02"))

	cleared, err := repo.Update(ctx, userA, e.ID, events.UpdateInput{ClearDate: true})
	require.NoError(t, err)
	require.Nil(t, cleared.EventDate)
}

func TestRepository_SetStatusAndCount(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Party", "party"))
	require.NoError(t, err)

	active, err := repo.SetStatus(ctx, userA, e.ID, events.StatusActive)
	require.NoError(t, err)
	require.Equal(t, events.StatusActive, active.Status)

	n, err := repo.CountByStatus(ctx, userA, events.StatusActive)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestRepository_ListFiltersAndPagination(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	// Insert 5 events with deterministic created_at to exercise keyset paging.
	for i := 0; i < 5; i++ {
		_, _, err := repo.Create(ctx, createInput(userA, "Event", "event-"+string(rune('a'+i))))
		require.NoError(t, err)
	}
	mustExec(t, pool, `UPDATE events SET created_at = '2026-01-01T00:00:00Z'`)

	page1, err := repo.List(ctx, userA, events.ListFilter{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	last := page1[len(page1)-1]
	cursor := &events.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	page2, err := repo.List(ctx, userA, events.ListFilter{Limit: 2, Cursor: cursor})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	seen := map[uuid.UUID]bool{}
	for _, e := range append(page1, page2...) {
		require.False(t, seen[e.ID], "duplicate event across pages")
		seen[e.ID] = true
	}
}

func TestRepository_ListStatusFilterAndSearch(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	_, _, err := repo.Create(ctx, createInput(userA, "Wedding Party", "wedding-party"))
	require.NoError(t, err)
	e2, _, err := repo.Create(ctx, createInput(userA, "Corporate Gala", "corporate-gala"))
	require.NoError(t, err)
	_, err = repo.SetStatus(ctx, userA, e2.ID, events.StatusActive)
	require.NoError(t, err)

	active := events.StatusActive
	activeItems, err := repo.List(ctx, userA, events.ListFilter{Status: &active})
	require.NoError(t, err)
	require.Len(t, activeItems, 1)
	require.Equal(t, "Corporate Gala", activeItems[0].Name)

	found, err := repo.List(ctx, userA, events.ListFilter{Query: "wedding"})
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, "Wedding Party", found[0].Name)
}

func TestRepository_SettingsTenantScoped(t *testing.T) {
	pool, userA, userB := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Party", "party"))
	require.NoError(t, err)

	_, err = repo.GetSettings(ctx, userB, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound)

	pwHash := "hashed"
	updated, err := repo.UpdateSettings(ctx, userA, e.ID, events.Settings{
		Visibility:   events.VisibilityPassword,
		PasswordHash: pwHash,
	})
	require.NoError(t, err)
	require.Equal(t, events.VisibilityPassword, updated.Visibility)
	require.Equal(t, pwHash, updated.PasswordHash)
}

func TestRepository_SettingsCascadeOnDelete(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Party", "party"))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, e.ID)
	require.NoError(t, err)

	var count int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM event_settings WHERE event_id = $1`, e.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

func strPtr(s string) *string { return &s }

func TestRepository_Extend(t *testing.T) {
	pool, userA, userB := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	future := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	in := createInput(userA, "Party", "party")
	in.ExpiresAt = &future
	e, _, err := repo.Create(ctx, in)
	require.NoError(t, err)

	extended, err := repo.Extend(ctx, userA, e.ID, future.AddDate(0, 0, 30), false)
	require.NoError(t, err)
	require.NotNil(t, extended.ExpiresAt)
	require.WithinDuration(t, future.AddDate(0, 0, 30), *extended.ExpiresAt, time.Second)

	_, err = repo.Extend(ctx, userB, e.ID, future, false)
	require.ErrorIs(t, err, events.ErrNotFound, "extend must be tenant scoped")
}

func TestRepository_ExtendReactivatesExpiredAndClearsWarning(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	in := createInput(userA, "Party", "party")
	past := time.Now().UTC().Add(-time.Hour)
	in.ExpiresAt = &past
	e, _, err := repo.Create(ctx, in)
	require.NoError(t, err)
	_, err = repo.SetStatus(ctx, userA, e.ID, events.StatusExpired)
	require.NoError(t, err)
	mustExec(t, pool, `UPDATE events SET expiry_warned_at = NOW() WHERE id = '`+e.ID.String()+`'`)

	extended, err := repo.Extend(ctx, userA, e.ID, time.Now().UTC().AddDate(0, 0, 7), true)
	require.NoError(t, err)
	require.Equal(t, events.StatusActive, extended.Status)

	var warned *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT expiry_warned_at FROM events WHERE id = $1`, e.ID).Scan(&warned))
	require.Nil(t, warned, "extend must reset the expiry warning")
}

func TestRepository_ListDueExpiry(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	due, _, err := repo.Create(ctx, createInput(userA, "Due", "due"))
	require.NoError(t, err)
	future, _, err := repo.Create(ctx, createInput(userA, "Future", "future"))
	require.NoError(t, err)
	archived, _, err := repo.Create(ctx, createInput(userA, "Archived", "archived"))
	require.NoError(t, err)

	mustExec(t, pool, `UPDATE events SET expires_at = NOW() - INTERVAL '1 hour', status = 'active' WHERE id = '`+due.ID.String()+`'`)
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() + INTERVAL '1 day', status = 'active' WHERE id = '`+future.ID.String()+`'`)
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() - INTERVAL '1 hour', status = 'archived' WHERE id = '`+archived.ID.String()+`'`)

	items, err := repo.ListDueExpiry(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, due.ID, items[0].ID)

	require.NoError(t, repo.MarkExpired(ctx, due.ID))
	got, err := repo.GetByID(ctx, userA, due.ID)
	require.NoError(t, err)
	require.Equal(t, events.StatusExpired, got.Status)
}

func TestRepository_ExpiryWarnings(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	soon, _, err := repo.Create(ctx, createInput(userA, "Soon", "soon"))
	require.NoError(t, err)
	later, _, err := repo.Create(ctx, createInput(userA, "Later", "later"))
	require.NoError(t, err)
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() + INTERVAL '3 days' WHERE id = '`+soon.ID.String()+`'`)
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() + INTERVAL '30 days' WHERE id = '`+later.ID.String()+`'`)

	warnings, err := repo.ListExpiryWarnings(ctx, now, now.AddDate(0, 0, 7), 10)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Equal(t, soon.ID, warnings[0].EventID)
	require.Equal(t, "a@example.com", warnings[0].OwnerEmail)

	require.NoError(t, repo.MarkExpiryWarned(ctx, soon.ID, now))
	warnings, err = repo.ListExpiryWarnings(ctx, now, now.AddDate(0, 0, 7), 10)
	require.NoError(t, err)
	require.Empty(t, warnings, "warned events must not warn twice")
}

func TestRepository_PurgeFlow(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Old", "old"))
	require.NoError(t, err)
	mustExec(t, pool, `UPDATE events SET storage_bytes = 4096 WHERE id = '`+e.ID.String()+`'`)
	mustExec(t, pool, `UPDATE users SET storage_bytes = 4096 WHERE id = '`+userA.String()+`'`)
	mustExec(t, pool, `INSERT INTO photos (event_id, storage_key, thumbnail_key, medium_key, optimized_key)
		VALUES ('`+e.ID.String()+`', 'orig-1', 'thumb-1', 'medium-1', 'optimized-1'),
		       ('`+e.ID.String()+`', 'orig-2', NULL, NULL, NULL)`)
	mustExec(t, pool, `INSERT INTO exports (event_id, status, object_key)
		VALUES ('`+e.ID.String()+`', 'ready', 'export-1')`)
	require.NoError(t, repo.SoftDelete(ctx, userA, e.ID))
	mustExec(t, pool, `UPDATE events SET deleted_at = NOW() - INTERVAL '40 days' WHERE id = '`+e.ID.String()+`'`)

	cutoff := time.Now().UTC().AddDate(0, 0, -30)
	candidates, err := repo.ListPurgeable(ctx, cutoff, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(4096), candidates[0].StorageBytes)

	keys, err := repo.PurgeKeys(ctx, e.ID)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"orig-1", "thumb-1", "medium-1", "optimized-1", "orig-2", "export-1"}, keys)

	deleted, err := repo.PurgeEvent(ctx, e.ID)
	require.NoError(t, err)
	require.True(t, deleted)

	var exportRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM exports WHERE event_id = $1`, e.ID).Scan(&exportRows))
	require.Zero(t, exportRows, "exports cascade with the event")

	var storage int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE id = $1`, userA).Scan(&storage))
	require.Zero(t, storage, "purge must decrement user storage once")
	_, err = repo.GetByID(ctx, userA, e.ID)
	require.ErrorIs(t, err, events.ErrNotFound)

	deleted, err = repo.PurgeEvent(ctx, e.ID)
	require.NoError(t, err)
	require.False(t, deleted, "second purge is a no-op")
	require.NoError(t, pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE id = $1`, userA).Scan(&storage))
	require.Zero(t, storage)
}

func TestRepository_ListPurgeableExpiredPastGrace(t *testing.T) {
	pool, userA, _ := setupDB(t)
	repo := events.NewRepository(pool)
	ctx := context.Background()

	e, _, err := repo.Create(ctx, createInput(userA, "Expired", "expired"))
	require.NoError(t, err)
	_, err = repo.SetStatus(ctx, userA, e.ID, events.StatusExpired)
	require.NoError(t, err)
	mustExec(t, pool, `UPDATE events SET expires_at = NOW() - INTERVAL '40 days' WHERE id = '`+e.ID.String()+`'`)

	cutoff := time.Now().UTC().AddDate(0, 0, -30)
	candidates, err := repo.ListPurgeable(ctx, cutoff, 10)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.Equal(t, e.ID, candidates[0].EventID)
}
