package gallery

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// SignedURLRateLimitKey scopes the signing budget to one client IP per event.
// Guests behind the same NAT share a bucket, but unrelated events do not, so
// one busy venue cannot exhaust another event's budget. clientIP must resolve
// the address through the trusted-proxy policy in pkg/httpx.
func SignedURLRateLimitKey(clientIP func(*http.Request) string) func(*http.Request) string {
	return func(r *http.Request) string {
		ip := clientIP(r)
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			return ip
		}
		return ip + "|" + slug
	}
}
