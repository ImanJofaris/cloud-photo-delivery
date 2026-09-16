package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/metrics"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *database.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://x:y@127.0.0.1:1/none")
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	// Constructing a pool does not connect until first use.
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return &database.Pool{Pool: pool}
}

func TestHealthz(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	var env struct {
		Data  map[string]string `json:"data"`
		Error any               `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data["status"] != "ok" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestReadyz_DatabaseDown(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHealthz_RequestIDHeader(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func TestProtectedRoute_RequiresAuth(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthRoute_RejectsInvalidJSON(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestRouter_SecurityHeaders(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS must not be set in non-prod, got %q", got)
	}
}

func TestRouter_BodyLimitReturns413(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 10, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	body := strings.NewReader(`{"email":"` + strings.Repeat("a", 1<<11) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestRouter_MetricsNotExposedPublicly(t *testing.T) {
	router := NewRouter(config.Config{Env: "test", CORSAllowedOrigins: []string{"*"}, MaxBodyBytes: 1 << 20, RequestTimeout: time.Second}, nil, testPool(t), metrics.New())

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected /metrics to be absent from the public router, got %d", rec.Code)
	}
}
