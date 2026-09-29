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

func TestRateLimiter_AllowNChargesCost(t *testing.T) {
	rl := NewRateLimiter(60, 10, time.Minute)
	if !rl.AllowN("key", 7) {
		t.Fatal("cost 7 within burst 10 should be allowed")
	}
	if !rl.AllowN("key", 3) {
		t.Fatal("cost 3 exactly draining the burst should be allowed")
	}
	if rl.AllowN("key", 1) {
		t.Fatal("bucket should be exhausted")
	}
}

func TestRateLimiter_CostAboveBurstAlwaysDenied(t *testing.T) {
	rl := NewRateLimiter(60, 5, time.Minute)
	if rl.AllowN("key", 6) {
		t.Fatal("a cost above the burst can never be admitted")
	}
}

func TestRateLimiter_AllowNTreatsZeroAsOne(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	if !rl.AllowN("key", 0) {
		t.Fatal("first request should be allowed")
	}
	if rl.AllowN("key", 0) {
		t.Fatal("zero cost must still charge one token")
	}
}

func TestRateLimiter_MiddlewareNChargesPayloadCost(t *testing.T) {
	rl := NewRateLimiter(60, 3, time.Minute)
	h := rl.MiddlewareN(
		func(r *http.Request) string { return "ip" },
		func(r *http.Request) int { return 2 },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/photos/urls", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("first request got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/photos/urls", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second request got %d, want 429 (2+2 > burst 3)", rec.Code)
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

func TestRateLimiter_RetryAfterReflectsRefill(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	base := time.Now()
	rl.now = func() time.Time { return base }

	h := rl.Middleware(func(r *http.Request) string { return "ip" })(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/auth/login", nil))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	// 60/min means one token per second, not the old fixed minute.
	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q, want \"1\"", got)
	}
}

func TestRateLimiter_ChargeDeniesWithRetryAfter(t *testing.T) {
	rl := NewRateLimiter(60, 1, time.Minute)
	base := time.Now()
	rl.now = func() time.Time { return base }

	rec := httptest.NewRecorder()
	if !rl.Charge(rec, "venue", 1) {
		t.Fatal("first charge should succeed")
	}
	rec = httptest.NewRecorder()
	if rl.Charge(rec, "venue", 1) {
		t.Fatal("second charge should be denied")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After")
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid envelope: %v", err)
	}
	if env.Error == nil || env.Error.Code != "RATE_LIMITED" {
		t.Fatalf("unexpected error: %+v", env.Error)
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
