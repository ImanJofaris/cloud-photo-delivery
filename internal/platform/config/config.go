package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
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

	CORSAllowedOrigins []string
	MaxBodyBytes       int64
	RequestTimeout     time.Duration

	MetricsAddr       string
	WorkerMetricsAddr string

	DBMaxConns           int
	DBMinConns           int
	DBConnMaxLifetime    time.Duration
	DBConnMaxIdleTime    time.Duration
	DBStatementTimeout   time.Duration
	DBSlowQueryThreshold time.Duration
}

func (c Config) IsProd() bool { return c.Env == "prod" }

// DatabaseOptions maps the DB_* environment knobs onto pool options.
func (c Config) DatabaseOptions(log *slog.Logger) database.Options {
	opts := database.DefaultOptions()
	opts.MaxConns = c.DBMaxConns
	opts.MinConns = c.DBMinConns
	opts.MaxConnLifetime = c.DBConnMaxLifetime
	opts.MaxConnIdleTime = c.DBConnMaxIdleTime
	opts.StatementTimeout = c.DBStatementTimeout
	opts.SlowQueryThreshold = c.DBSlowQueryThreshold
	opts.Logger = log
	return opts
}

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

		CORSAllowedOrigins: getList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		MaxBodyBytes:       getInt64("MAX_BODY_BYTES", 1<<20),
		RequestTimeout:     getDuration("HTTP_REQUEST_TIMEOUT", 30*time.Second),

		MetricsAddr:       getEnv("METRICS_ADDR", ":9091"),
		WorkerMetricsAddr: getEnv("WORKER_METRICS_ADDR", ":9092"),

		DBMaxConns:           getInt("DB_MAX_CONNS", 10),
		DBMinConns:           getInt("DB_MIN_CONNS", 1),
		DBConnMaxLifetime:    getDuration("DB_CONN_MAX_LIFETIME", time.Hour),
		DBConnMaxIdleTime:    getDuration("DB_CONN_MAX_IDLE_TIME", 30*time.Minute),
		DBStatementTimeout:   getDuration("DB_STATEMENT_TIMEOUT", 30*time.Second),
		DBSlowQueryThreshold: getDuration("DB_SLOW_QUERY_THRESHOLD", 500*time.Millisecond),
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
	if c.MaxBodyBytes <= 0 {
		errs = append(errs, "MAX_BODY_BYTES must be positive")
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, "HTTP_REQUEST_TIMEOUT must be positive")
	}
	if c.DBMaxConns < 1 {
		errs = append(errs, "DB_MAX_CONNS must be at least 1")
	}
	if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
		errs = append(errs, "DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
	}
	if c.DBStatementTimeout < 0 {
		errs = append(errs, "DB_STATEMENT_TIMEOUT must not be negative")
	}
	if c.DBSlowQueryThreshold < 0 {
		errs = append(errs, "DB_SLOW_QUERY_THRESHOLD must not be negative")
	}
	if c.IsProd() {
		for _, o := range c.CORSAllowedOrigins {
			if o == "*" {
				errs = append(errs, "CORS_ALLOWED_ORIGINS must not contain * in prod")
				break
			}
		}
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

func getInt64(key string, fallback int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getList(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
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
