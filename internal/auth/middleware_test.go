package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRequireAuth_MissingHeader(t *testing.T) {
	ts := NewTokenService("secret", time.Minute, time.Hour)
	svc := &Service{tokens: ts}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	})
	rec := httptest.NewRecorder()
	svc.RequireAuth(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAuth_MalformedHeader(t *testing.T) {
	ts := NewTokenService("secret", time.Minute, time.Hour)
	svc := &Service{tokens: ts}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc")
	svc.RequireAuth(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	ts := NewTokenService("secret", time.Minute, time.Hour)
	svc := &Service{tokens: ts}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	svc.RequireAuth(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestRequireAuth_ValidTokenSetsUserID(t *testing.T) {
	ts := NewTokenService("secret", time.Minute, time.Hour)
	svc := &Service{tokens: ts}
	id := uuid.New()
	token, _ := ts.IssueAccessToken(id)

	var got uuid.UUID
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	svc.RequireAuth(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if got != id {
		t.Fatalf("got %s want %s", got, id)
	}
}
