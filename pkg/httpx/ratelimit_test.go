package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewRateLimiter(60, 5, time.Minute)
	for i := 0; i < 5; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	rl := NewRateLimiter(60, 3, time.Minute)
	for i := 0; i < 3; i++ {
		rl.Allow("1.2.3.4")
	}
	if rl.Allow("1.2.3.4") {
		t.Fatal("expected over-limit request to be blocked")
	}
}

func TestRateLimiter_SeparateKeys(t *testing.T) {
	rl := NewRateLimiter(60, 2, time.Minute)
	rl.Allow("a")
	rl.Allow("a")
	if rl.Allow("a") {
		t.Fatal("key a should be limited")
	}
	if !rl.Allow("b") {
		t.Fatal("key b should not be limited")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	h := rl.Middleware(func(r *http.Request) string { return "ip" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("first request got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request got %d, want 429", rec.Code)
	}
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	if got := ClientIP(req); got != "9.9.9.9" {
		t.Fatalf("got %q", got)
	}

	req.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	if got := ClientIP(req); got != "1.1.1.1" {
		t.Fatalf("got %q", got)
	}
}
