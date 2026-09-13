package devices

import (
	"context"
	"net/http"
	"strings"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type contextKey string

const deviceKey contextKey = "device"

func WithDevice(ctx context.Context, d *Device) context.Context {
	return context.WithValue(ctx, deviceKey, d)
}

func FromContext(ctx context.Context) (*Device, bool) {
	d, ok := ctx.Value(deviceKey).(*Device)
	return d, ok
}

// RequireDevice authenticates requests that carry a device key only.
func (s *Service) RequireDevice(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := deviceCredential(r)
		if !ok {
			httpx.Error(w, r, apperr.Unauthorized())
			return
		}
		device, err := s.Authenticate(r.Context(), raw)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithDevice(r.Context(), device)))
	})
}

// RequireDeviceOrOperator authenticates device-key requests and delegates all
// other requests to the operator JWT middleware.
func (s *Service) RequireDeviceOrOperator(operator func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := deviceCredential(r)
			if !ok {
				if operator == nil {
					httpx.Error(w, r, apperr.Unauthorized())
					return
				}
				operator(next).ServeHTTP(w, r)
				return
			}
			device, err := s.Authenticate(r.Context(), raw)
			if err != nil {
				httpx.Error(w, r, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithDevice(r.Context(), device)))
		})
	}
}

// RateLimit throttles device-authenticated requests per device; operator
// requests pass through untouched.
func RateLimit(rl *httpx.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			device, ok := FromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			if !rl.Allow("device:" + device.ID.String()) {
				w.Header().Set("Retry-After", "60")
				httpx.Fail(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func deviceCredential(r *http.Request) (string, bool) {
	if key := strings.TrimSpace(r.Header.Get("X-Api-Key")); key != "" {
		return key, true
	}
	h := r.Header.Get("Authorization")
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Device") && parts[1] != "" {
		return parts[1], true
	}
	return "", false
}
