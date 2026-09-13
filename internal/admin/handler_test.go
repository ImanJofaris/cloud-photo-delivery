package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "body=%s", rec.Body.String())
	return env
}

func newTestHandler(repo *fakeRepo) *Handler {
	return NewHandler(NewService(repo))
}

func TestHandler_Stats(t *testing.T) {
	repo := newFakeRepo()
	repo.stats = &Stats{Users: 5, Events: 11, Photos: 220, StorageBytes: 4096, RevenueCents: 15000, Subscriptions: 3}
	rec := httptest.NewRecorder()
	newTestHandler(repo).Stats(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, float64(5), data["users"])
	require.Equal(t, float64(11), data["events"])
	require.Equal(t, float64(220), data["photos"])
	require.Equal(t, float64(4096), data["storageBytes"])
	require.Equal(t, float64(15000), data["revenueCents"])
	require.Equal(t, float64(3), data["subscriptions"])
}

func TestHandler_Users(t *testing.T) {
	repo := newFakeRepo()
	created := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	repo.users = []*UserSummary{{
		ID:           uuid.New(),
		Email:        "operator@example.com",
		BusinessName: "Acme Photobooth",
		IsAdmin:      true,
		StorageBytes: 2048,
		EventCount:   4,
		CreatedAt:    created,
	}}
	rec := httptest.NewRecorder()
	newTestHandler(repo).Users(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?limit=20", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Nil(t, data["nextCursor"])
	users := data["users"].([]any)
	require.Len(t, users, 1)
	u := users[0].(map[string]any)
	require.Equal(t, "operator@example.com", u["email"])
	require.Equal(t, "Acme Photobooth", u["businessName"])
	require.Equal(t, true, u["isAdmin"])
	require.Equal(t, float64(2048), u["storageBytes"])
	require.Equal(t, float64(4), u["eventCount"])
}

func TestHandler_Users_EmptyIsArray(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(newFakeRepo()).Users(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"users":[]`)
	require.Contains(t, rec.Body.String(), `"nextCursor":null`)
}

func TestHandler_Users_InvalidCursor(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(newFakeRepo()).Users(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?cursor=zzz", nil))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "VALIDATION_ERROR", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_Subscriptions(t *testing.T) {
	repo := newFakeRepo()
	periodEnd := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	repo.subs = []*SubscriptionSummary{{
		ID:               uuid.New(),
		UserID:           uuid.New(),
		UserEmail:        "operator@example.com",
		PlanID:           "pro",
		Status:           "active",
		Interval:         "month",
		CurrentPeriodEnd: &periodEnd,
		CreatedAt:        time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC),
	}}
	rec := httptest.NewRecorder()
	newTestHandler(repo).Subscriptions(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	subs := data["subscriptions"].([]any)
	require.Len(t, subs, 1)
	s := subs[0].(map[string]any)
	require.Equal(t, "pro", s["planId"])
	require.Equal(t, "active", s["status"])
	require.Equal(t, "month", s["interval"])
	require.Equal(t, false, s["cancelAtPeriodEnd"])
	require.NotNil(t, s["currentPeriodEnd"])
}

func TestHandler_Subscriptions_EmptyIsArray(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler(newFakeRepo()).Subscriptions(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/subscriptions", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"subscriptions":[]`)
}

func TestHandler_Health(t *testing.T) {
	repo := newFakeRepo()
	oldest := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	repo.queue = &QueueHealth{Pending: 2, Running: 1, Failed: 1, OldestPendingAt: &oldest}
	rec := httptest.NewRecorder()
	newTestHandler(repo).Health(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/health", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, StatusDegraded, data["status"])
	require.Equal(t, float64(3), data["queueDepth"])
	queue := data["queue"].(map[string]any)
	require.Equal(t, float64(2), queue["pending"])
	require.Equal(t, float64(1), queue["running"])
	require.Equal(t, float64(1), queue["failed"])
	require.NotNil(t, queue["oldestPendingAt"])
}

func TestParseLimit(t *testing.T) {
	require.Equal(t, 0, parseLimit(""))
	require.Equal(t, 0, parseLimit("abc"))
	require.Equal(t, 25, parseLimit("25"))
}
