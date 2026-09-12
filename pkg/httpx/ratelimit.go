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
	return v.limiter.AllowN(now, 1)
}

func (rl *RateLimiter) cleanup(now time.Time) {
	for k, v := range rl.byKey {
		if now.Sub(v.lastSeen) > rl.ttl {
			delete(rl.byKey, k)
		}
	}
}

func (rl *RateLimiter) Middleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !rl.Allow(keyFn(r)) {
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
