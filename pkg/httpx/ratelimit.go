package httpx

import (
	"math"
	"net/http"
	"strconv"
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
	_, ok := rl.chargeN(key, n)
	return ok
}

// Charge is AllowN for handlers that only learn the cost after resolving a
// payload. When the budget is exhausted it writes the standard 429 envelope
// with a Retry-After that reflects the actual refill delay.
func (rl *RateLimiter) Charge(w http.ResponseWriter, key string, n int) bool {
	delay, ok := rl.chargeN(key, n)
	if ok {
		return true
	}
	seconds := int(math.Ceil(delay.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	Fail(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please try again later")
	return false
}

// chargeN reports whether n units may proceed and, when denied, how long
// until enough tokens refill. The reservation is rolled back on denial so a
// refused request never accrues debt.
func (rl *RateLimiter) chargeN(key string, n int) (time.Duration, bool) {
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

	reservation := v.limiter.ReserveN(now, n)
	if !reservation.OK() {
		return 60 * time.Second, false
	}
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)
		return delay, false
	}
	return 0, true
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
// Endpoints whose cost depends on a resolved payload should charge the
// remainder with Charge inside the handler.
func (rl *RateLimiter) MiddlewareN(keyFn func(*http.Request) string, costFn func(*http.Request) int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cost := 1
			if costFn != nil {
				cost = costFn(r)
			}
			if !rl.Charge(w, keyFn(r), cost) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
