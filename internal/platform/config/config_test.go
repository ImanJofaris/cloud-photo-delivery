package config

import (
	"testing"
	"time"
)

func TestLoad_Valid(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "test" || cfg.DatabaseURL != "postgres://x" || cfg.HTTPAddr != ":9090" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.BillingProvider != "manual" {
		t.Fatalf("expected default billing provider manual, got %q", cfg.BillingProvider)
	}
	if cfg.ShutdownTimeout != 5*time.Second {
		t.Fatalf("unexpected shutdown timeout: %v", cfg.ShutdownTimeout)
	}
	if cfg.IsProd() {
		t.Fatal("test env should not be prod")
	}
}

func TestLoad_MissingJWTSecret(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when JWT_SECRET missing")
	}
}

func TestLoad_MissingBillingWebhookSecret(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when BILLING_WEBHOOK_SECRET missing")
	}
}

func TestLoad_InvalidBillingProvider(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
	t.Setenv("BILLING_PROVIDER", "stripe")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for unsupported BILLING_PROVIDER")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when DATABASE_URL missing")
	}
}

func TestLoad_InvalidEnv(t *testing.T) {
	t.Setenv("APP_ENV", "staging")
	t.Setenv("DATABASE_URL", "postgres://x")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid APP_ENV")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("expected default addr, got %q", cfg.HTTPAddr)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected default log level, got %q", cfg.LogLevel)
	}
	if cfg.R2Region != "auto" {
		t.Errorf("expected default region auto, got %q", cfg.R2Region)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("expected default access TTL 15m, got %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("expected default refresh TTL 720h, got %v", cfg.RefreshTokenTTL)
	}
	if cfg.LockoutMaxFailed != 5 {
		t.Errorf("expected default lockout max 5, got %d", cfg.LockoutMaxFailed)
	}
	if cfg.LockoutDuration != 15*time.Minute {
		t.Errorf("expected default lockout duration 15m, got %v", cfg.LockoutDuration)
	}
	if cfg.PublicBaseURL != "http://localhost:3000" {
		t.Errorf("expected default public base url, got %q", cfg.PublicBaseURL)
	}
	if cfg.SignedURLTTL != 5*time.Minute {
		t.Errorf("expected default signed URL TTL 5m, got %v", cfg.SignedURLTTL)
	}
	if cfg.GalleryUnlockTTL != 30*time.Minute {
		t.Errorf("expected default gallery unlock TTL 30m, got %v", cfg.GalleryUnlockTTL)
	}
	if cfg.EventPurgeGraceDays != 30 {
		t.Errorf("expected default purge grace 30d, got %d", cfg.EventPurgeGraceDays)
	}
	if cfg.ExpiryWarnDays != 7 {
		t.Errorf("expected default expiry warning 7d, got %d", cfg.ExpiryWarnDays)
	}
	if cfg.ExportTTL != 24*time.Hour {
		t.Errorf("expected default export TTL 24h, got %v", cfg.ExportTTL)
	}
}

func TestLoad_ExportTTLOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
	t.Setenv("EXPORT_TTL", "2h30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ExportTTL != 2*time.Hour+30*time.Minute {
		t.Fatalf("unexpected export TTL: %v", cfg.ExportTTL)
	}
}

func TestLoad_NonPositiveExportTTLRejected(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
	t.Setenv("EXPORT_TTL", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive EXPORT_TTL")
	}
}

func TestLoad_LifecycleOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
	t.Setenv("EVENT_PURGE_GRACE_DAYS", "14")
	t.Setenv("EXPIRY_WARN_DAYS", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EventPurgeGraceDays != 14 || cfg.ExpiryWarnDays != 3 {
		t.Fatalf("unexpected lifecycle config: %+v", cfg)
	}
}

func TestLoad_NegativeLifecycleValuesRejected(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
	t.Setenv("EVENT_PURGE_GRACE_DAYS", "-1")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative EVENT_PURGE_GRACE_DAYS")
	}
}

func TestLoad_AnalyticsSalt(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AnalyticsSalt != "test-secret" {
		t.Fatalf("expected analytics salt to fall back to JWT secret, got %q", cfg.AnalyticsSalt)
	}

	t.Setenv("ANALYTICS_HASH_SALT", "analytics-secret")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AnalyticsSalt != "analytics-secret" {
		t.Fatalf("expected overridden analytics salt, got %q", cfg.AnalyticsSalt)
	}
}

func TestValidateStorage(t *testing.T) {
	cfg := Config{R2Endpoint: "http://x", R2AccessKey: "a", R2SecretKey: "b", R2Bucket: "c"}
	if err := cfg.ValidateStorage(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := (Config{}).ValidateStorage(); err == nil {
		t.Fatal("expected error for incomplete storage config")
	}
}

func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("BILLING_WEBHOOK_SECRET", "billing-secret")
}

func TestLoad_HardeningDefaults(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("MAX_BODY_BYTES", "")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "")
	t.Setenv("METRICS_ADDR", "")
	t.Setenv("WORKER_METRICS_ADDR", "")
	t.Setenv("DB_MAX_CONNS", "")
	t.Setenv("DB_MIN_CONNS", "")
	t.Setenv("DB_STATEMENT_TIMEOUT", "")
	t.Setenv("DB_SLOW_QUERY_THRESHOLD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CORSAllowedOrigins) != 1 || cfg.CORSAllowedOrigins[0] != "*" {
		t.Errorf("expected default CORS wildcard, got %v", cfg.CORSAllowedOrigins)
	}
	if cfg.MaxBodyBytes != 1<<20 {
		t.Errorf("expected 1 MiB default body limit, got %d", cfg.MaxBodyBytes)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("expected 30s request timeout, got %v", cfg.RequestTimeout)
	}
	if cfg.MetricsAddr != ":9091" || cfg.WorkerMetricsAddr != ":9092" {
		t.Errorf("unexpected metrics addrs: %q %q", cfg.MetricsAddr, cfg.WorkerMetricsAddr)
	}
	if cfg.DBMaxConns != 10 || cfg.DBMinConns != 1 {
		t.Errorf("unexpected pool bounds: %d %d", cfg.DBMaxConns, cfg.DBMinConns)
	}
	if cfg.DBStatementTimeout != 30*time.Second {
		t.Errorf("unexpected statement timeout: %v", cfg.DBStatementTimeout)
	}
	if cfg.DBSlowQueryThreshold != 500*time.Millisecond {
		t.Errorf("unexpected slow query threshold: %v", cfg.DBSlowQueryThreshold)
	}
}

func TestLoad_CORSListParsed(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.CORSAllowedOrigins) != 2 {
		t.Fatalf("expected 2 origins, got %v", cfg.CORSAllowedOrigins)
	}
	if cfg.CORSAllowedOrigins[0] != "https://app.example.com" || cfg.CORSAllowedOrigins[1] != "https://admin.example.com" {
		t.Fatalf("unexpected origins: %v", cfg.CORSAllowedOrigins)
	}
}

func TestLoad_ProdRejectsWildcardCORS(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for wildcard CORS in prod")
	}
}

func TestLoad_ProdAllowsExplicitCORS(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com")

	if _, err := Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_InvalidDBPoolBounds(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DB_MAX_CONNS", "5")
	t.Setenv("DB_MIN_CONNS", "10")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when DB_MIN_CONNS exceeds DB_MAX_CONNS")
	}
}

func TestLoad_InvalidBodyAndTimeout(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("MAX_BODY_BYTES", "0")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for non-positive body limit and timeout")
	}
}
