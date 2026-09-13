//go:build integration

package billing_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/uploads"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupBillingDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
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

	mustExec(t, pool, `CREATE TABLE plans (
		id VARCHAR(40) PRIMARY KEY,
		name VARCHAR(80) NOT NULL,
		price_cents INT NOT NULL,
		currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
		interval VARCHAR(10) NOT NULL DEFAULT 'month',
		limits JSONB NOT NULL,
		active BOOLEAN NOT NULL DEFAULT TRUE
	)`)
	mustExec(t, pool, `INSERT INTO plans (id, name, price_cents, limits) VALUES
		('free', 'Free', 0, '{"activeEvents":1,"photosPerEvent":1,"storageBytes":100000,"retentionDays":7,"apiAccess":false}'::jsonb),
		('starter', 'Starter', 2900, '{"activeEvents":5,"photosPerEvent":10,"storageBytes":1000000,"retentionDays":30,"apiAccess":false}'::jsonb),
		('pro', 'Pro', 5900, '{"activeEvents":0,"photosPerEvent":20,"storageBytes":5000000,"retentionDays":90,"apiAccess":true}'::jsonb)`)

	mustExec(t, pool, `CREATE TABLE subscriptions (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		plan_id VARCHAR(40) NOT NULL REFERENCES plans(id),
		status VARCHAR(30) NOT NULL,
		provider VARCHAR(30),
		provider_ref TEXT,
		interval VARCHAR(10) NOT NULL DEFAULT 'month',
		current_period_start TIMESTAMPTZ,
		current_period_end TIMESTAMPTZ,
		cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE UNIQUE INDEX idx_subs_non_terminal_user ON subscriptions(user_id)
		WHERE status IN ('trialing', 'active', 'past_due')`)
	mustExec(t, pool, `CREATE UNIQUE INDEX idx_subs_provider_ref ON subscriptions(provider_ref)
		WHERE provider_ref IS NOT NULL`)

	mustExec(t, pool, `CREATE TABLE invoices (
		id UUID PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		subscription_id UUID REFERENCES subscriptions(id) ON DELETE SET NULL,
		amount_cents INT NOT NULL,
		currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
		status VARCHAR(20) NOT NULL,
		provider_ref TEXT,
		issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		paid_at TIMESTAMPTZ
	)`)
	mustExec(t, pool, `CREATE INDEX idx_invoices_user_issued ON invoices(user_id, issued_at DESC, id DESC)`)

	mustExec(t, pool, `CREATE TABLE billing_webhook_events (
		provider VARCHAR(30) NOT NULL,
		provider_ref TEXT NOT NULL,
		event_type VARCHAR(60) NOT NULL,
		received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (provider, provider_ref)
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

func mustExec(t *testing.T, pool *pgxpool.Pool, sql string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql)
	require.NoError(t, err)
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

func activeInput(userID uuid.UUID, ref string) billing.CreateSubscriptionInput {
	now := time.Now().UTC()
	return billing.CreateSubscriptionInput{
		ID:                 uuid.New(),
		UserID:             userID,
		PlanID:             "starter",
		Status:             billing.StatusActive,
		Provider:           "manual",
		ProviderRef:        ref,
		Interval:           "month",
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
	}
}

type stubStore struct{}

func (stubStore) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return "https://example.test/put", nil
}
func (stubStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://example.test/get", nil
}
func (stubStore) Head(context.Context, string) (int64, error)       { return 0, nil }
func (stubStore) Delete(context.Context, string) error              { return nil }
func (stubStore) Put(context.Context, string, string, []byte) error { return nil }
func (stubStore) Get(context.Context, string) ([]byte, error)       { return nil, nil }
func (stubStore) Bucket() string                                    { return "test" }
func (stubStore) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "multipart-id", nil
}
func (stubStore) PresignUploadPart(context.Context, string, string, int, time.Duration) (string, error) {
	return "https://example.test/part", nil
}
func (stubStore) CompleteMultipartUpload(context.Context, string, string, []r2.CompletePart) error {
	return nil
}
func (stubStore) AbortMultipartUpload(context.Context, string, string) error { return nil }

func TestBillingRepository_SubscriptionLifecycle(t *testing.T) {
	pool, userA, userB := setupBillingDB(t)
	repo := billing.NewRepository(pool)
	ctx := context.Background()

	plan, err := repo.GetPlan(ctx, "free")
	require.NoError(t, err)
	require.Equal(t, 1, plan.Limits.ActiveEvents)
	require.EqualValues(t, 100000, plan.Limits.StorageBytes)

	sub, err := repo.CreateSubscription(ctx, activeInput(userA, "ref-a"))
	require.NoError(t, err)
	require.Equal(t, billing.StatusActive, sub.Status)
	require.NotNil(t, sub.ProviderRef)

	fetched, err := repo.GetActiveSubscription(ctx, userA)
	require.NoError(t, err)
	require.Equal(t, sub.ID, fetched.ID)

	_, err = repo.GetActiveSubscription(ctx, userB)
	require.ErrorIs(t, err, billing.ErrNotFound, "tenant isolation")

	updated, err := repo.UpdateSubscriptionPlan(ctx, sub.ID, "pro", "month")
	require.NoError(t, err)
	require.Equal(t, "pro", updated.PlanID)

	canceled, err := repo.SetCancelAtPeriodEnd(ctx, sub.ID, true)
	require.NoError(t, err)
	require.True(t, canceled.CancelAtPeriodEnd)

	expired, err := repo.SetStatus(ctx, sub.ID, billing.StatusExpired)
	require.NoError(t, err)
	require.Equal(t, billing.StatusExpired, expired.Status)

	_, err = repo.GetActiveSubscription(ctx, userA)
	require.ErrorIs(t, err, billing.ErrNotFound)
}

func TestBillingRepository_UniqueActiveSubscriptionPerUser(t *testing.T) {
	pool, userA, _ := setupBillingDB(t)
	repo := billing.NewRepository(pool)
	ctx := context.Background()

	first, err := repo.CreateSubscription(ctx, activeInput(userA, "ref-1"))
	require.NoError(t, err)

	_, err = repo.CreateSubscription(ctx, activeInput(userA, "ref-2"))
	require.ErrorIs(t, err, billing.ErrActiveSubscriptionExists)

	_, err = repo.SetStatus(ctx, first.ID, billing.StatusCanceled)
	require.NoError(t, err)

	second, err := repo.CreateSubscription(ctx, activeInput(userA, "ref-3"))
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
}

func TestBillingRepository_WebhookIdempotencyConstraint(t *testing.T) {
	pool, _, _ := setupBillingDB(t)
	repo := billing.NewRepository(pool)
	ctx := context.Background()

	inserted, err := repo.RecordWebhookEvent(ctx, "manual", "evt-1", "invoice.paid")
	require.NoError(t, err)
	require.True(t, inserted)

	inserted, err = repo.RecordWebhookEvent(ctx, "manual", "evt-1", "invoice.paid")
	require.NoError(t, err)
	require.False(t, inserted, "same provider event id is ignored")

	inserted, err = repo.RecordWebhookEvent(ctx, "manual", "evt-2", "invoice.paid")
	require.NoError(t, err)
	require.True(t, inserted)
}

func TestBillingRepository_InvoicePagination(t *testing.T) {
	pool, userA, _ := setupBillingDB(t)
	repo := billing.NewRepository(pool)
	ctx := context.Background()

	base := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	for i, id := range ids {
		_, err := pool.Exec(ctx,
			`INSERT INTO invoices (id, user_id, amount_cents, status, issued_at) VALUES ($1, $2, 100, 'open', $3)`,
			id, userA, base.Add(time.Duration(i)*time.Minute))
		require.NoError(t, err)
	}

	first, err := repo.ListInvoices(ctx, userA, nil, 2)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.Equal(t, ids[2], first[0].ID)

	cursor := billing.InvoiceCursor{IssuedAt: first[1].IssuedAt, ID: first[1].ID}
	second, err := repo.ListInvoices(ctx, userA, &cursor, 2)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, ids[0], second[0].ID)
}

func TestBillingEntitlements_BlockEventCreationAtBoundary(t *testing.T) {
	pool, userA, _ := setupBillingDB(t)
	ctx := context.Background()
	repo := billing.NewRepository(pool)
	ent := billing.NewEntitlements(repo)
	eventSvc := events.NewService(events.NewRepository(pool), ent,
		func(string) (string, error) { return "hash", nil })

	_, _, err := eventSvc.Create(ctx, userA, events.CreateParams{Name: "First", Status: events.StatusActive})
	require.NoError(t, err)

	_, _, err = eventSvc.Create(ctx, userA, events.CreateParams{Name: "Second", Status: events.StatusActive})
	require.Error(t, err)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "PLAN_LIMIT_REACHED", appErr.Code)
	require.Contains(t, appErr.Message, "activeEvents")

	_, _, err = eventSvc.Create(ctx, userA, events.CreateParams{Name: "Upcoming"})
	require.NoError(t, err, "upcoming events are not limited")

	_, err = repo.CreateSubscription(ctx, activeInput(userA, "ref-lift"))
	require.NoError(t, err)
	_, _, err = eventSvc.Create(ctx, userA, events.CreateParams{Name: "Third", Status: events.StatusActive})
	require.NoError(t, err, "starter lifts the active-event limit")
}

func TestBillingEntitlements_BlockUploadInitAtBoundary(t *testing.T) {
	pool, userA, userB := setupBillingDB(t)
	ctx := context.Background()
	repo := billing.NewRepository(pool)
	ent := billing.NewEntitlements(repo)
	eventA := insertEvent(t, pool, userA, "a")
	eventB := insertEvent(t, pool, userB, "b")

	uploadSvc := uploads.NewService(photos.NewRepository(pool), uploads.NewRepository(pool), stubStore{}, nil, ent)

	_, err := uploadSvc.Initialize(ctx, uploads.Actor{UserID: userA}, uploads.InitParams{
		EventID: eventA, Filename: "a.jpg", ContentType: "image/jpeg", Size: 100,
	})
	require.NoError(t, err)

	_, err = uploadSvc.Initialize(ctx, uploads.Actor{UserID: userA}, uploads.InitParams{
		EventID: eventA, Filename: "b.jpg", ContentType: "image/jpeg", Size: 100,
	})
	require.Error(t, err)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "PLAN_LIMIT_REACHED", appErr.Code)
	require.Contains(t, appErr.Message, "photosPerEvent")

	_, err = pool.Exec(ctx, `UPDATE users SET storage_bytes = 99950 WHERE id = $1`, userB)
	require.NoError(t, err)
	_, err = uploadSvc.Initialize(ctx, uploads.Actor{UserID: userB}, uploads.InitParams{
		EventID: eventB, Filename: "c.jpg", ContentType: "image/jpeg", Size: 100,
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "PLAN_LIMIT_REACHED", appErr.Code)
	require.Contains(t, appErr.Message, "storageBytes")

	_, err = uploadSvc.Initialize(ctx, uploads.Actor{UserID: userB}, uploads.InitParams{
		EventID: eventB, Filename: "d.jpg", ContentType: "image/jpeg", Size: 10,
	})
	require.NoError(t, err, "within the storage limit succeeds")
}

func TestBillingRepository_InvoiceAndUsageRoundTrip(t *testing.T) {
	pool, userA, _ := setupBillingDB(t)
	repo := billing.NewRepository(pool)
	ctx := context.Background()

	plans, err := repo.ListPlans(ctx)
	require.NoError(t, err)
	require.Len(t, plans, 3)
	require.Equal(t, "free", plans[0].ID)

	sub, err := repo.CreateSubscription(ctx, activeInput(userA, "ref-rt"))
	require.NoError(t, err)

	byRef, err := repo.GetSubscriptionByProviderRef(ctx, "ref-rt")
	require.NoError(t, err)
	require.Equal(t, sub.ID, byRef.ID)

	latest, err := repo.GetLatestSubscription(ctx, userA)
	require.NoError(t, err)
	require.Equal(t, sub.ID, latest.ID)

	invoice, err := repo.CreateInvoice(ctx, billing.CreateInvoiceInput{
		ID: uuid.New(), UserID: userA, SubscriptionID: &sub.ID,
		AmountCents: 2900, Currency: "MYR", Status: billing.InvoiceOpen, ProviderRef: "inv-rt",
	})
	require.NoError(t, err)

	byInvoiceRef, err := repo.GetInvoiceByProviderRef(ctx, "inv-rt")
	require.NoError(t, err)
	require.Equal(t, invoice.ID, byInvoiceRef.ID)

	open, err := repo.GetLatestOpenInvoice(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, invoice.ID, open.ID)

	require.NoError(t, repo.MarkInvoicePaid(ctx, invoice.ID, time.Now().UTC()))
	paid, err := repo.GetInvoiceByProviderRef(ctx, "inv-rt")
	require.NoError(t, err)
	require.Equal(t, billing.InvoicePaid, paid.Status)
	require.NotNil(t, paid.PaidAt)

	_, err = repo.GetLatestOpenInvoice(ctx, sub.ID)
	require.ErrorIs(t, err, billing.ErrNotFound)
	require.ErrorIs(t, repo.MarkInvoicePaid(ctx, uuid.New(), time.Now()), billing.ErrNotFound)
	_, err = repo.GetInvoiceByProviderRef(ctx, "missing")
	require.ErrorIs(t, err, billing.ErrNotFound)

	inserted, err := repo.RecordWebhookEvent(ctx, "manual", "evt-rt", "invoice.paid")
	require.NoError(t, err)
	require.True(t, inserted)
	require.NoError(t, repo.DeleteWebhookEvent(ctx, "manual", "evt-rt"))
	inserted, err = repo.RecordWebhookEvent(ctx, "manual", "evt-rt", "invoice.paid")
	require.NoError(t, err)
	require.True(t, inserted, "deleted idempotency rows can be recorded again")

	eventID := insertEvent(t, pool, userA, "active-event")
	_, err = pool.Exec(ctx, `UPDATE events SET status = 'active' WHERE id = $1`, eventID)
	require.NoError(t, err)
	count, err := repo.CountActiveEvents(ctx, userA)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	_, err = pool.Exec(ctx, `UPDATE users SET storage_bytes = 42 WHERE id = $1`, userA)
	require.NoError(t, err)
	bytes, err := repo.UserStorageBytes(ctx, userA)
	require.NoError(t, err)
	require.EqualValues(t, 42, bytes)

	_, err = repo.UserStorageBytes(ctx, uuid.New())
	require.ErrorIs(t, err, billing.ErrNotFound)

	inserted, err = repo.RecordWebhookEvent(ctx, "billplz", "evt-rt", "invoice.paid")
	require.NoError(t, err)
	require.True(t, inserted, "idempotency is scoped by provider")
}

func TestBillingRepository_NotFoundIsDistinct(t *testing.T) {
	pool, _, _ := setupBillingDB(t)
	repo := billing.NewRepository(pool)

	_, err := repo.GetPlan(context.Background(), "nonexistent")
	require.True(t, errors.Is(err, billing.ErrNotFound))
}
