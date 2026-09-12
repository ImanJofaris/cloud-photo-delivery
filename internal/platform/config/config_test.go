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

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != "test" || cfg.DatabaseURL != "postgres://x" || cfg.HTTPAddr != ":9090" {
		t.Fatalf("unexpected config: %+v", cfg)
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
