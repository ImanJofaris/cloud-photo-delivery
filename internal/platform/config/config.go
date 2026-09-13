package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	HTTPAddr    string
	LogLevel    string
	DatabaseURL string

	R2Endpoint  string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
	R2Region    string

	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	PasswordResetTTL time.Duration
	LockoutMaxFailed int
	LockoutDuration  time.Duration
	PublicBaseURL    string

	SignedURLTTL     time.Duration
	GalleryUnlockTTL time.Duration
	AnalyticsSalt    string

	BillingProvider      string
	BillingWebhookSecret string

	EventPurgeGraceDays int
	ExpiryWarnDays      int
	ExportTTL           time.Duration

	ShutdownTimeout time.Duration

	WorkerConcurrency int
	WorkerPollEvery   time.Duration
}

func (c Config) IsProd() bool { return c.Env == "prod" }

func Load() (Config, error) {
	c := Config{
		Env:             getEnv("APP_ENV", "dev"),
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		R2Endpoint:      os.Getenv("R2_ENDPOINT"),
		R2AccessKey:     os.Getenv("R2_ACCESS_KEY"),
		R2SecretKey:     os.Getenv("R2_SECRET_KEY"),
		R2Bucket:        os.Getenv("R2_BUCKET"),
		R2Region:        getEnv("R2_REGION", "auto"),
		ShutdownTimeout: getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),

		WorkerConcurrency: getInt("WORKER_CONCURRENCY", 2),
		WorkerPollEvery:   getDuration("WORKER_POLL_INTERVAL", time.Second),

		JWTSecret:        os.Getenv("JWT_SECRET"),
		AccessTokenTTL:   getDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:  getDuration("REFRESH_TOKEN_TTL", 720*time.Hour),
		PasswordResetTTL: getDuration("PASSWORD_RESET_TTL", time.Hour),
		LockoutMaxFailed: getInt("LOCKOUT_MAX_ATTEMPTS", 5),
		LockoutDuration:  getDuration("LOCKOUT_DURATION", 15*time.Minute),
		PublicBaseURL:    getEnv("PUBLIC_BASE_URL", "http://localhost:3000"),

		SignedURLTTL:     getDuration("SIGNED_URL_TTL", 5*time.Minute),
		GalleryUnlockTTL: getDuration("GALLERY_UNLOCK_TTL", 30*time.Minute),
		AnalyticsSalt:    os.Getenv("ANALYTICS_HASH_SALT"),

		BillingProvider:      getEnv("BILLING_PROVIDER", "manual"),
		BillingWebhookSecret: os.Getenv("BILLING_WEBHOOK_SECRET"),

		EventPurgeGraceDays: getInt("EVENT_PURGE_GRACE_DAYS", 30),
		ExpiryWarnDays:      getInt("EXPIRY_WARN_DAYS", 7),
		ExportTTL:           getDuration("EXPORT_TTL", 24*time.Hour),
	}

	if err := c.validate(); err != nil {
		return Config{}, err
	}
	if c.AnalyticsSalt == "" {
		c.AnalyticsSalt = c.JWTSecret
	}
	return c, nil
}

func (c Config) validate() error {
	var errs []string

	switch c.Env {
	case "dev", "test", "prod":
	default:
		errs = append(errs, "APP_ENV must be one of dev|test|prod")
	}

	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		errs = append(errs, "JWT_SECRET is required")
	}
	if c.LockoutMaxFailed < 1 {
		errs = append(errs, "LOCKOUT_MAX_ATTEMPTS must be at least 1")
	}
	switch c.BillingProvider {
	case "manual":
	default:
		errs = append(errs, "BILLING_PROVIDER must be one of manual")
	}
	if c.BillingWebhookSecret == "" {
		errs = append(errs, "BILLING_WEBHOOK_SECRET is required")
	}
	if c.EventPurgeGraceDays < 0 {
		errs = append(errs, "EVENT_PURGE_GRACE_DAYS must not be negative")
	}
	if c.ExpiryWarnDays < 0 {
		errs = append(errs, "EXPIRY_WARN_DAYS must not be negative")
	}
	if c.ExportTTL <= 0 {
		errs = append(errs, "EXPORT_TTL must be positive")
	}
	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}
	return nil
}

func (c Config) ValidateStorage() error {
	if c.R2Endpoint == "" || c.R2AccessKey == "" || c.R2SecretKey == "" || c.R2Bucket == "" {
		return errors.New("invalid config: R2_ENDPOINT, R2_ACCESS_KEY, R2_SECRET_KEY and R2_BUCKET are required")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		if n, err2 := strconv.Atoi(v); err2 == nil {
			return time.Duration(n) * time.Second
		}
		return fallback
	}
	return d
}

func (c Config) String() string {
	return fmt.Sprintf("env=%s addr=%s bucket=%s", c.Env, c.HTTPAddr, c.R2Bucket)
}
