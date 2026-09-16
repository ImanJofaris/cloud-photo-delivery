//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

type billingAPI struct {
	handler http.Handler
	pool    *pgxpool.Pool
	manual  *provider.Manual
}

func setupBillingAPI(t *testing.T) *billingAPI {
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
		('free', 'Free', 0, '{"activeEvents":1,"photosPerEvent":500,"storageBytes":5368709120,"retentionDays":7,"apiAccess":false}'::jsonb),
		('starter', 'Starter', 2900, '{"activeEvents":5,"photosPerEvent":5000,"storageBytes":53687091200,"retentionDays":30,"apiAccess":false}'::jsonb)`)
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
	mustExec(t, pool, `CREATE TABLE billing_webhook_events (
		provider VARCHAR(30) NOT NULL,
		provider_ref TEXT NOT NULL,
		event_type VARCHAR(60) NOT NULL,
		received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		PRIMARY KEY (provider, provider_ref)
	)`)

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

	billingRepo := billing.NewRepository(pool)
	manual := provider.NewManual("e2e-billing-secret")
	billingSvc := billing.NewService(billingRepo, manual)
	billingHandler := billing.NewHandler(billingSvc, manual, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})
	entitlements := billing.NewEntitlements(billingRepo)

	eventRepo := events.NewRepository(pool)
	eventSvc := events.NewService(eventRepo, entitlements, auth.HashPassword)
	eventHandler := events.NewHandler(eventSvc, func(r *http.Request) (string, bool) {
		id, ok := auth.UserID(r.Context())
		if !ok {
			return "", false
		}
		return id.String(), true
	})

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/signup", authHandler.Signup)
		r.Post("/billing/webhook", billingHandler.Webhook)
		r.Group(func(r chi.Router) {
			r.Use(authSvc.RequireAuth)
			r.Post("/events", eventHandler.Create)
			r.Route("/billing", func(r chi.Router) {
				r.Get("/plans", billingHandler.Plans)
				r.Get("/subscription", billingHandler.GetSubscription)
				r.Post("/subscribe", billingHandler.Subscribe)
				r.Post("/upgrade", billingHandler.Upgrade)
				r.Post("/downgrade", billingHandler.Downgrade)
				r.Post("/cancel", billingHandler.Cancel)
				r.Post("/resume", billingHandler.Resume)
				r.Get("/invoices", billingHandler.Invoices)
			})
		})
	})
	return &billingAPI{handler: r, pool: pool, manual: manual}
}

func TestE2E_BillingFlow(t *testing.T) {
	api := setupBillingAPI(t)

	rec := doReq(t, api.handler, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"bill@example.com","password":"password123","businessName":"Booth"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	access, _ := parseAuth(t, rec)

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/events", `{"name":"One","status":"active"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/events", `{"name":"Two","status":"active"}`, access)
	require.Equal(t, http.StatusPaymentRequired, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "PLAN_LIMIT_REACHED")
	require.Contains(t, rec.Body.String(), "activeEvents")

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	var subscribeEnv struct {
		Data struct {
			Subscription struct {
				ProviderRef string `json:"providerRef"`
				Status      string `json:"status"`
			} `json:"subscription"`
			Checkout struct {
				URL string `json:"url"`
			} `json:"checkout"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &subscribeEnv))
	require.Equal(t, "active", subscribeEnv.Data.Subscription.Status)
	require.Contains(t, subscribeEnv.Data.Checkout.URL, "manual://checkout/")
	require.NotEmpty(t, subscribeEnv.Data.Subscription.ProviderRef)

	webhookBody := `{"id":"evt-e2e-1","type":"invoice.paid","providerRef":"` +
		subscribeEnv.Data.Subscription.ProviderRef + `"}`
	rec = doReqHeaders(t, api.handler, http.MethodPost, "/api/v1/billing/webhook", webhookBody, "",
		map[string]string{provider.SignatureHeader: api.manual.Sign([]byte(webhookBody))})
	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, api.handler, http.MethodGet, "/api/v1/billing/invoices", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"paid"`)

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/events", `{"name":"Two","status":"active"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "starter lifts the active-event limit: %s", rec.Body.String())

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/billing/cancel", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"cancelAtPeriodEnd":true`)
	require.Contains(t, rec.Body.String(), `"status":"active"`)

	rec = doReq(t, api.handler, http.MethodGet, "/api/v1/billing/subscription", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"active"`, "access remains until period end")

	_, err := api.pool.Exec(context.Background(),
		`UPDATE subscriptions SET current_period_end = NOW() - interval '1 day'`)
	require.NoError(t, err)

	rec = doReq(t, api.handler, http.MethodGet, "/api/v1/billing/subscription", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"expired"`)
	require.Contains(t, rec.Body.String(), `"id":"free"`, "expired subscription falls back to free")

	rec = doReq(t, api.handler, http.MethodPost, "/api/v1/events", `{"name":"Three","status":"active"}`, access)
	require.Equal(t, http.StatusPaymentRequired, rec.Code, "free limits apply again after expiry")
	require.Contains(t, rec.Body.String(), "PLAN_LIMIT_REACHED")
}
