package httpx

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// HTTPObserver receives per-request metrics. The metrics package implements
// it; tests use a fake.
type HTTPObserver interface {
	ObserveHTTP(method, route string, status int, duration time.Duration)
	IncInFlight()
	DecInFlight()
}

// Metrics records request counts, latency, and in-flight requests. The route
// label is the chi route pattern (e.g. /api/v1/events/{eventID}/uploads), so
// label cardinality stays bounded.
func Metrics(obs HTTPObserver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			obs.IncInFlight()
			defer obs.DecInFlight()

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			obs.ObserveHTTP(normalizeMethod(r.Method), RoutePattern(r), status, time.Since(start))
		})
	}
}

// normalizeMethod bounds the method label: the server accepts arbitrary
// method tokens, and each unique value creates metric children that are never
// evicted.
func normalizeMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions:
		return method
	default:
		return "OTHER"
	}
}

// RoutePattern returns the matched chi route pattern, or "unmatched".
func RoutePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if pattern := rc.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}
