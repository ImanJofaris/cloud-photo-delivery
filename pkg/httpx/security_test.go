package httpx

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSecurityHeaders_Prod(t *testing.T) {
	handler := SecurityHeaders(true)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("unexpected X-Content-Type-Options: %q", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("unexpected X-Frame-Options: %q", got)
	}
	if got := rec.Header().Get("Referrer-Policy"); got != "strict-origin-when-cross-origin" {
		t.Fatalf("unexpected Referrer-Policy: %q", got)
	}
	if got := rec.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=31536000") {
		t.Fatalf("missing HSTS in prod: %q", got)
	}
}

func TestSecurityHeaders_DevOmitsHSTS(t *testing.T) {
	handler := SecurityHeaders(false)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("HSTS should not be set outside prod, got %q", got)
	}
}

func TestMaxBytes_RejectsOversizedBody(t *testing.T) {
	var readErr error
	handler := MaxBytes(16)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 64)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("expected MaxBytesError, got %v", readErr)
	}
}

func TestRequestTimeout_PassesFastHandler(t *testing.T) {
	handler := RequestTimeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Success(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequestTimeout_Returns504Envelope(t *testing.T) {
	handler := RequestTimeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error == nil || env.Error.Code != "REQUEST_TIMEOUT" {
		t.Fatalf("unexpected error body: %+v", env.Error)
	}
}

func TestRequestTimeout_DiscardsLateWrites(t *testing.T) {
	late := make(chan struct{})
	handler := RequestTimeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		_, _ = w.Write([]byte("late"))
		close(late)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	<-late

	if strings.Contains(rec.Body.String(), "late") {
		t.Fatalf("late write leaked into response: %q", rec.Body.String())
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error == nil || env.Error.Code != "REQUEST_TIMEOUT" {
		t.Fatalf("unexpected error body: %+v", env.Error)
	}
}

func TestRequestTimeout_KeepsEarlyResponse(t *testing.T) {
	handler := RequestTimeout(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Success(w, http.StatusCreated, map[string]string{"id": "1"})
		<-r.Context().Done()
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
}

func TestError_MapsDeadlineExceededTo504(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, context.DeadlineExceeded)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504, got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error == nil || env.Error.Code != "REQUEST_TIMEOUT" {
		t.Fatalf("unexpected error body: %+v", env.Error)
	}
}
