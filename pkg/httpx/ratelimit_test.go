package httpx

import (
	"encoding/json"
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

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	base := time.Now()
	rl.now = func() time.Time { return base }

	if !rl.Allow("key") {
		t.Fatal("first request should be allowed")
	}
	if rl.Allow("key") {
		t.Fatal("second request should be blocked (burst 1)")
	}

	base = base.Add(2 * time.Second)
	if !rl.Allow("key") {
		t.Fatal("request after refill window should be allowed")
	}
}

func TestRateLimiter_MiddlewareEnvelope(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	h := rl.Middleware(func(r *http.Request) string { return "ip" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/auth/login", nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error == nil || env.Error.Code != "RATE_LIMITED" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
