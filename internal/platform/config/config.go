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

	ShutdownTimeout time.Duration
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
	}

	if err := c.validate(); err != nil {
		return Config{}, err
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
