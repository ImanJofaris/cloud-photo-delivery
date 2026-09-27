package httpx

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimiter struct {
	limit rate.Limit
	burst int
	ttl   time.Duration
	mu    sync.Mutex
	byKey map[string]*visitor
	now   func() time.Time
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(perMinute int, burst int, ttl time.Duration) *RateLimiter {
	if burst <= 0 {
		burst = perMinute
	}
	return &RateLimiter{
		limit: rate.Limit(float64(perMinute) / 60.0),
		burst: burst,
		ttl:   ttl,
		byKey: make(map[string]*visitor),
		now:   time.Now,
	}
}

func (rl *RateLimiter) Allow(key string) bool {
	return rl.AllowN(key, 1)
}

// AllowN reports whether n units may proceed for key, charging n tokens. A
// cost above the burst size can never be admitted. Costs below 1 are treated
// as 1 so a request is never free.
func (rl *RateLimiter) AllowN(key string, n int) bool {
	if n < 1 {
		n = 1
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.byKey[key]
	now := rl.now()
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.byKey[key] = v
	}
	v.lastSeen = now

	if rl.ttl > 0 {
		rl.cleanup(now)
	}
	return v.limiter.AllowN(now, n)
}

func (rl *RateLimiter) cleanup(now time.Time) {
	for k, v := range rl.byKey {
		if now.Sub(v.lastSeen) > rl.ttl {
			delete(rl.byKey, k)
		}
	}
}

func (rl *RateLimiter) Middleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return rl.MiddlewareN(keyFn, nil)
}

// MiddlewareN is Middleware with a per-request cost. A nil costFn costs 1.
// Use it for endpoints whose cost varies with the payload (for example batch
// signing, where one request signs many URLs).
func (rl *RateLimiter) MiddlewareN(keyFn func(*http.Request) string, costFn func(*http.Request) int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cost := 1
			if costFn != nil {
				cost = costFn(r)
			}
			if !rl.AllowN(keyFn(r), cost) {
				w.Header().Set("Retry-After", "60")
				Fail(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := indexByte(xff, ','); i >= 0 {
			return trimSpace(xff[:i])
		}
		return trimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
