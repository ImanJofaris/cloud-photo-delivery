//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/admin"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type fakeReconcileReporter struct {
	reports []admin.ReconcileReport
}

func (f *fakeReconcileReporter) ReportReconcile(_ context.Context, report admin.ReconcileReport) error {
	f.reports = append(f.reports, report)
	return nil
}

func adminExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

func setupAdminFlowAPI(t *testing.T) (http.Handler, *pgxpool.Pool, *r2.S3Store) {
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
		status VARCHAR(30) NOT NULL DEFAULT 'upcoming',
		storage_bytes BIGINT NOT NULL DEFAULT 0,
		photo_count BIGINT NOT NULL DEFAULT 0,
		guest_count BIGINT NOT NULL DEFAULT 0,
		expires_at TIMESTAMPTZ,
		expiry_warned_at TIMESTAMPTZ,
		deleted_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT events_slug_unique UNIQUE (user_id, slug)
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
	mustExec(t, pool, `CREATE TABLE subscriptions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		plan_id VARCHAR(50) NOT NULL,
		status VARCHAR(20) NOT NULL,
		provider VARCHAR(30),
		provider_ref TEXT,
		interval VARCHAR(20) NOT NULL DEFAULT 'month',
		current_period_start TIMESTAMPTZ,
		current_period_end TIMESTAMPTZ,
		cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	mustExec(t, pool, `CREATE TABLE invoices (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		subscription_id UUID,
		amount_cents INTEGER NOT NULL,
		currency VARCHAR(3) NOT NULL DEFAULT 'MYR',
		status VARCHAR(20) NOT NULL,
		provider_ref TEXT,
		issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		paid_at TIMESTAMPTZ
	)`)
	mustExec(t, pool, `CREATE TABLE jobs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		type VARCHAR(50) NOT NULL,
		payload JSONB NOT NULL DEFAULT '{}'::jsonb,
		status VARCHAR(20) NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		last_error TEXT,
		run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		locked_at TIMESTAMPTZ,
		locked_by TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)

	endpoint := startMinioE2E(t)
	createBucketE2E(t, endpoint, "cpd-admin")
	store, err := r2.New(ctx, r2.Options{
		Endpoint:  endpoint,
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "cpd-admin",
		Region:    "us-east-1",
		UseSSL:    false,
	})
	require.NoError(t, err)

	userRepo := users.NewRepository(pool)
	authSvc := auth.NewService(userRepo, auth.NewRepository(pool),
		auth.NewTokenService("e2e-secret", 15*time.Minute, 30*24*time.Hour), &auth.LogMailer{}, auth.Config{
			AccessTTL:        15 * time.Minute,
			RefreshTTL:       30 * 24 * time.Hour,
			PasswordResetTTL: time.Hour,
			LockoutMaxFailed: 5,
			LockoutDuration:  15 * time.Minute,
			PublicBaseURL:    "http://localhost:3000",
		})
	authHandler := auth.NewHandler(authSvc)

	adminSvc := admin.NewService(admin.NewRepository(pool))
	adminHandler := admin.NewHandler(adminSvc)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth, adminSvc.RequireAdmin)
			r.Get("/admin/stats", adminHandler.Stats)
			r.Get("/admin/users", adminHandler.Users)
			r.Get("/admin/subscriptions", adminHandler.Subscriptions)
			r.Get("/admin/health", adminHandler.Health)
		})
	})
	return r, pool, store
}

func TestE2E_AdminFlow(t *testing.T) {
	h, pool, store := setupAdminFlowAPI(t)
	ctx := context.Background()

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"admin@example.com","password":"password123","businessName":"Admin Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	adminAccess, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"operator@example.com","password":"password123","businessName":"Operator Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	operatorAccess, _ := parseAuth(t, rec)

	adminExec(t, pool, `UPDATE users SET is_admin = TRUE WHERE email = 'admin@example.com'`)

	// Seed one event with one ready photo, a paid invoice, an active
	// subscription, and a failed job for another tenant.
	var adminID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM users WHERE email = 'admin@example.com'`).Scan(&adminID))
	var eventID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO events (user_id, name, slug, status, photo_count, storage_bytes)
		 VALUES ($1, 'Admin Party', 'admin-party', 'active', 1, 3) RETURNING id`, adminID).Scan(&eventID))
	storageKey := r2.OriginalKey(adminID, eventID, uuid.New(), "a.jpg")
	adminExec(t, pool,
		`INSERT INTO photos (event_id, storage_key, mime_type, file_size, status)
		 VALUES ($1, $2, 'image/jpeg', 3, 'READY')`, eventID, storageKey)
	adminExec(t, pool, `UPDATE users SET storage_bytes = 3 WHERE id = $1`, adminID)
	adminExec(t, pool,
		`INSERT INTO invoices (user_id, amount_cents, status) VALUES ($1, 4900, 'paid')`, adminID)
	adminExec(t, pool,
		`INSERT INTO subscriptions (user_id, plan_id, status, interval, current_period_end)
		 VALUES ($1, 'pro', 'active', 'month', NOW() + INTERVAL '30 days')`, adminID)
	adminExec(t, pool,
		`INSERT INTO jobs (type, status) VALUES ('zip.generate', 'failed'), ('event.expire', 'pending')`)

	// Unauthenticated and non-admin callers are rejected.
	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/stats", "", "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/stats", "", operatorAccess)
	require.Equal(t, http.StatusForbidden, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "FORBIDDEN")

	// Admin stats aggregate the seeded fixtures.
	var statsEnv struct {
		Data struct {
			Users         int64 `json:"users"`
			Events        int64 `json:"events"`
			Photos        int64 `json:"photos"`
			StorageBytes  int64 `json:"storageBytes"`
			RevenueCents  int64 `json:"revenueCents"`
			Subscriptions int64 `json:"subscriptions"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/stats", "", adminAccess)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &statsEnv))
	require.EqualValues(t, 2, statsEnv.Data.Users)
	require.EqualValues(t, 1, statsEnv.Data.Events)
	require.EqualValues(t, 1, statsEnv.Data.Photos)
	require.EqualValues(t, 3, statsEnv.Data.StorageBytes)
	require.EqualValues(t, 4900, statsEnv.Data.RevenueCents)
	require.EqualValues(t, 1, statsEnv.Data.Subscriptions)

	// Users paginate newest first and expose the admin flag.
	var usersEnv struct {
		Data struct {
			Users []struct {
				Email   string `json:"email"`
				IsAdmin bool   `json:"isAdmin"`
			} `json:"users"`
			NextCursor *string `json:"nextCursor"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/users?limit=1", "", adminAccess)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &usersEnv))
	require.Len(t, usersEnv.Data.Users, 1)
	require.Equal(t, "operator@example.com", usersEnv.Data.Users[0].Email)
	require.False(t, usersEnv.Data.Users[0].IsAdmin)
	require.NotNil(t, usersEnv.Data.NextCursor)

	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/users?limit=1&cursor="+*usersEnv.Data.NextCursor, "", adminAccess)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &usersEnv))
	require.Len(t, usersEnv.Data.Users, 1)
	require.Equal(t, "admin@example.com", usersEnv.Data.Users[0].Email)
	require.True(t, usersEnv.Data.Users[0].IsAdmin)

	// Subscriptions list across tenants.
	var subsEnv struct {
		Data struct {
			Subscriptions []struct {
				PlanID string `json:"planId"`
				Status string `json:"status"`
			} `json:"subscriptions"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/subscriptions", "", adminAccess)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &subsEnv))
	require.Len(t, subsEnv.Data.Subscriptions, 1)
	require.Equal(t, "pro", subsEnv.Data.Subscriptions[0].PlanID)
	require.Equal(t, "active", subsEnv.Data.Subscriptions[0].Status)

	// Health reports the failed job as degraded.
	var healthEnv struct {
		Data struct {
			Status     string `json:"status"`
			QueueDepth int64  `json:"queueDepth"`
			Queue      struct {
				Pending int64 `json:"pending"`
				Running int64 `json:"running"`
				Failed  int64 `json:"failed"`
			} `json:"queue"`
		} `json:"data"`
	}
	rec = doReq(t, h, http.MethodGet, "/api/v1/admin/health", "", adminAccess)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &healthEnv))
	require.Equal(t, admin.StatusDegraded, healthEnv.Data.Status)
	require.EqualValues(t, 1, healthEnv.Data.QueueDepth)
	require.EqualValues(t, 1, healthEnv.Data.Queue.Pending)
	require.EqualValues(t, 1, healthEnv.Data.Queue.Failed)

	// Reconciliation matches the counters to the stored original, then
	// detects an injected orphan.
	require.NoError(t, store.Put(ctx, storageKey, "image/jpeg", []byte("abc")))
	reporter := &fakeReconcileReporter{}
	reconcile := admin.ReconcileHandler(admin.NewRepository(pool), store, time.Now, reporter)
	require.NoError(t, reconcile(ctx, nil))
	require.Len(t, reporter.reports, 1)
	require.True(t, reporter.reports[0].InSync(), "report=%+v", reporter.reports[0])

	orphanKey := r2.OriginalKey(adminID, eventID, uuid.New(), "orphan.jpg")
	require.NoError(t, store.Put(ctx, orphanKey, "image/jpeg", []byte("hello")))
	require.NoError(t, reconcile(ctx, nil))
	require.Len(t, reporter.reports, 2)
	require.False(t, reporter.reports[1].InSync())
	require.EqualValues(t, 5, reporter.reports[1].DriftBytes())
	require.EqualValues(t, 1, reporter.reports[1].DriftObjects())

	// The reconcile job must tolerate a missing object without deleting
	// anything: removing the orphan restores the sync state.
	require.NoError(t, store.Delete(ctx, orphanKey))
	require.NoError(t, reconcile(ctx, nil))
	require.True(t, reporter.reports[2].InSync())

	_, err := store.Head(ctx, storageKey)
	require.NoError(t, err, "reconcile never deletes originals")
}
