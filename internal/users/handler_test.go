package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func decodeProfileEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env.Data
}

func TestHandler_Me_IncludesIsAdminFalseForNormalUser(t *testing.T) {
	repo := newFakeRepo()
	u, err := repo.Create(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "a@b.com", "hash", "Booth")
	require.NoError(t, err)

	h := NewHandler(NewService(repo), func(*http.Request) (string, bool) {
		return u.ID.String(), true
	})

	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest(http.MethodGet, "/account/me", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"isAdmin":false`)
	data := decodeProfileEnvelope(t, rec)
	require.Equal(t, false, data["isAdmin"])
}

func TestHandler_Me_IncludesIsAdminTrueForAdmin(t *testing.T) {
	repo := newFakeRepo()
	u, err := repo.Create(httptest.NewRequest(http.MethodGet, "/", nil).Context(), "admin@b.com", "hash", "Booth")
	require.NoError(t, err)
	u.IsAdmin = true

	h := NewHandler(NewService(repo), func(*http.Request) (string, bool) {
		return u.ID.String(), true
	})

	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest(http.MethodGet, "/account/me", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"isAdmin":true`)
}

func TestHandler_Me_UnknownUserIsUnauthorized(t *testing.T) {
	h := NewHandler(NewService(newFakeRepo()), func(*http.Request) (string, bool) {
		return uuid.New().String(), true
	})

	rec := httptest.NewRecorder()
	h.Me(rec, httptest.NewRequest(http.MethodGet, "/account/me", nil))

	require.Equal(t, http.StatusNotFound, rec.Code, "body=%s", rec.Body.String())
}
