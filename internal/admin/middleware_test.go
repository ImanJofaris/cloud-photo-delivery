package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/stretchr/testify/require"
)

func adminChain(t *testing.T, repo *fakeRepo) (http.Handler, uuid.UUID, func() string) {
	t.Helper()
	tokens := auth.NewTokenService("test-secret", time.Minute, time.Hour)
	authSvc := auth.NewService(nil, nil, tokens, nil, auth.Config{})
	svc := NewService(repo)

	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"ok":true},"error":null}`))
	})
	handler := authSvc.RequireAuth(svc.RequireAdmin(protected))

	userID := uuid.New()
	issue := func() string {
		token, err := tokens.IssueAccessToken(userID)
		require.NoError(t, err)
		return token
	}
	return handler, userID, issue
}

func TestRequireAdmin_AllowsAdmin(t *testing.T) {
	repo := newFakeRepo()
	handler, userID, issue := adminChain(t, repo)
	repo.admins[userID] = true

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+issue())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
}

func TestRequireAdmin_ForbidsNonAdmin(t *testing.T) {
	repo := newFakeRepo()
	handler, _, issue := adminChain(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+issue())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, "body=%s", rec.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, "FORBIDDEN", env["error"].(map[string]any)["code"])
}

func TestRequireAdmin_RequiresAuth(t *testing.T) {
	handler, _, _ := adminChain(t, newFakeRepo())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireAdmin_RepoErrorIsInternal(t *testing.T) {
	repo := newFakeRepo()
	repo.isAdminErr = errBoom
	handler, _, issue := adminChain(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+issue())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	strBody := rec.Body.String()
	require.NotContains(t, strBody, "boom")
	require.Contains(t, strBody, "INTERNAL_ERROR")
}
