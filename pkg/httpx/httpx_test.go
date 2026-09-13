package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

func decode(t *testing.T, body []byte) Envelope {
	t.Helper()
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	return env
}

func TestSuccess_Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, http.StatusOK, map[string]string{"status": "ok"})

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", ct)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error != nil {
		t.Fatal("error should be nil on success")
	}
	if env.Data == nil {
		t.Fatal("data should not be nil")
	}
}

func TestFail_Envelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Fail(rec, http.StatusNotFound, "NOT_FOUND", "nope")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Data != nil {
		t.Fatal("data should be nil on error")
	}
	if env.Error == nil || env.Error.Code != "NOT_FOUND" {
		t.Fatalf("unexpected error body: %+v", env.Error)
	}
}

func TestError_MapsAppError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, apperr.Validation())

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("unexpected code: %s", env.Error.Code)
	}
}

func TestError_HidesInternalError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()

	Error(rec, req, context.DeadlineExceeded)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected code: %s", env.Error.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("context deadline")) {
		t.Fatal("internal error detail leaked to client")
	}
}

func TestRequestID_GeneratesAndEchoes(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetRequestID(r.Context()) == "" {
			t.Error("expected request id in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header")
	}
}

func TestRequestID_PreservesIncoming(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "abc123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "abc123" {
		t.Fatalf("got %q", got)
	}
}

func TestRecover_TurnsPanicInto500(t *testing.T) {
	handler := RequestID(Recover(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
	env := decode(t, rec.Body.Bytes())
	if env.Error == nil || env.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("unexpected body: %+v", env.Error)
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := CORS([]string{"*"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("missing CORS header")
	}
	if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "X-Gallery-Unlock") {
		t.Fatal("missing gallery unlock header in CORS allowlist")
	}
}

func TestCORS_AllowlistRejectsUnknownOrigin(t *testing.T) {
	handler := CORS([]string{"https://allowed.com"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://evil.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("unknown origin should not be allowed")
	}
}

func TestLogging_InjectsLoggerAndRequestID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	handler := RequestID(Logging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetRequestID(r.Context()) == "" {
			t.Error("missing request id")
		}
		w.WriteHeader(http.StatusOK)
	})))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}
