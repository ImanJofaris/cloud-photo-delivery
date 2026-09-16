package httpx

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

// SecurityHeaders sets conservative response headers for the JSON API. HSTS is
// only emitted in prod, where TLS terminates in front of the API.
func SecurityHeaders(prod bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if prod {
				h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBytes caps every request body. Handlers still apply tighter limits for
// JSON payloads; this is the backstop for any route that reads a body.
func MaxBytes(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequestTimeout bounds the time a handler may spend before its context is
// cancelled. If nothing has been written when the deadline passes, the client
// gets a 504 envelope; late writes from the handler are discarded.
func RequestTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()

			tw := &timeoutWriter{w: w, ctx: ctx}
			done := make(chan struct{})
			panicChan := make(chan any, 1)

			go func() {
				defer func() {
					if p := recover(); p != nil {
						panicChan <- p
					}
				}()
				next.ServeHTTP(tw, r.WithContext(ctx))
				close(done)
			}()

			select {
			case p := <-panicChan:
				panic(p)
			case <-done:
			case <-ctx.Done():
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					tw.timeout()
				}
			}
		})
	}
}

type timeoutWriter struct {
	mu          sync.Mutex
	w           http.ResponseWriter
	ctx         context.Context
	timedOut    bool
	wroteHeader bool
}

func (tw *timeoutWriter) Header() http.Header { return tw.w.Header() }

func (tw *timeoutWriter) expired() bool {
	return tw.timedOut || errors.Is(tw.ctx.Err(), context.DeadlineExceeded)
}

func (tw *timeoutWriter) WriteHeader(status int) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.expired() || tw.wroteHeader {
		tw.timedOut = true
		return
	}
	tw.wroteHeader = true
	tw.w.WriteHeader(status)
}

func (tw *timeoutWriter) Write(b []byte) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.expired() {
		tw.timedOut = true
		return 0, http.ErrHandlerTimeout
	}
	if !tw.wroteHeader {
		tw.wroteHeader = true
		tw.w.WriteHeader(http.StatusOK)
	}
	return tw.w.Write(b)
}

func (tw *timeoutWriter) timeout() {
	tw.mu.Lock()
	defer tw.mu.Unlock()
	if tw.wroteHeader {
		tw.timedOut = true
		return
	}
	tw.timedOut = true
	tw.wroteHeader = true
	Fail(tw.w, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "Request timed out")
}
