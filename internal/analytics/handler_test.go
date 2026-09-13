package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "body=%s", rec.Body.String())
	return env
}

func testHandler() (*Handler, uuid.UUID, *fakeRepo) {
	userID := uuid.New()
	svc, repo, _ := testService()
	h := NewHandler(svc, func(r *http.Request) (string, bool) {
		return userID.String(), true
	})
	return h, userID, repo
}

func TestHandler_Event(t *testing.T) {
	h, _, repo := testHandler()
	eventID := uuid.New()
	repo.summary = EventReport{EventID: eventID, PhotoCount: 12, Totals: Counters{GalleryViews: 10, UniqueVisitors: 4, Downloads: 2, QRScans: 3}}
	repo.daily = []DayCounters{
		{Day: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), Counters: Counters{GalleryViews: 10, UniqueVisitors: 4, Downloads: 2, QRScans: 3}},
	}

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/analytics?days=7", nil), "eventID", eventID.String())
	rec := httptest.NewRecorder()
	h.Event(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, eventID.String(), data["eventId"])
	require.Equal(t, float64(12), data["photoCount"])
	totals := data["totals"].(map[string]any)
	require.Equal(t, float64(10), totals["galleryViews"])
	require.Equal(t, float64(4), totals["uniqueVisitors"])
	require.Equal(t, float64(2), totals["downloads"])
	require.Equal(t, float64(3), totals["qrScans"])
	daily := data["daily"].([]any)
	require.Len(t, daily, 1)
	require.Equal(t, "2026-09-13", daily[0].(map[string]any)["date"])
}

func TestHandler_Event_EmptyDailyIsArray(t *testing.T) {
	h, _, repo := testHandler()
	eventID := uuid.New()
	repo.summary = EventReport{EventID: eventID}

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/analytics", nil), "eventID", eventID.String())
	rec := httptest.NewRecorder()
	h.Event(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"daily":[]`)
}

func TestHandler_Event_InvalidDays(t *testing.T) {
	h, _, _ := testHandler()
	eventID := uuid.New()

	for _, raw := range []string{"0", "366", "-1", "abc", "1.5"} {
		req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/analytics?days="+raw, nil), "eventID", eventID.String())
		rec := httptest.NewRecorder()
		h.Event(rec, req)
		require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "days=%s body=%s", raw, rec.Body.String())
	}
}

func TestHandler_Event_InvalidEventID(t *testing.T) {
	h, _, _ := testHandler()
	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/not-a-uuid/analytics", nil), "eventID", "not-a-uuid")
	rec := httptest.NewRecorder()
	h.Event(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "EVENT_NOT_FOUND", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_Event_Unauthenticated(t *testing.T) {
	svc, _, _ := testService()
	h := NewHandler(svc, func(r *http.Request) (string, bool) { return "", false })
	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+uuid.NewString()+"/analytics", nil), "eventID", uuid.NewString())
	rec := httptest.NewRecorder()
	h.Event(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Account(t *testing.T) {
	h, _, repo := testHandler()
	repo.account = AccountReport{EventCount: 3, PhotoCount: 50, Totals: Counters{GalleryViews: 7}}
	repo.accountDaily = []DayCounters{{Day: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), Counters: Counters{GalleryViews: 7}}}

	rec := httptest.NewRecorder()
	h.Account(rec, httptest.NewRequest(http.MethodGet, "/api/v1/account/analytics", nil))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, float64(3), data["eventCount"])
	require.Equal(t, float64(50), data["photoCount"])
	require.Equal(t, float64(7), data["totals"].(map[string]any)["galleryViews"])
	require.Len(t, data["daily"].([]any), 1)
}

func TestHandler_Account_InvalidDays(t *testing.T) {
	h, _, _ := testHandler()
	rec := httptest.NewRecorder()
	h.Account(rec, httptest.NewRequest(http.MethodGet, "/api/v1/account/analytics?days=999", nil))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestParseDays(t *testing.T) {
	days, err := parseDays("")
	require.NoError(t, err)
	require.Equal(t, 0, days)

	days, err = parseDays(" 14 ")
	require.NoError(t, err)
	require.Equal(t, 14, days)

	_, err = parseDays("nope")
	require.Error(t, err)
}
