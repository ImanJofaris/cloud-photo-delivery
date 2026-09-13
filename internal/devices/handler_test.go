package devices

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTestHandler(t *testing.T) (*Handler, *fakeRepo, uuid.UUID) {
	t.Helper()
	svc, repo, userID := newTestService(t)
	h := NewHandler(svc, func(*http.Request) (string, bool) { return userID.String(), true })
	return h, repo, userID
}

func withURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env
}

func TestHandler_Create_ReturnsKeyOnce(t *testing.T) {
	h, repo, userID := newTestHandler(t)
	eventID := uuid.New()
	repo.events[eventID] = userID

	body := `{"name":"Booth 1","assignedEventId":"` + eventID.String() + `"}`
	r := httptest.NewRequest(http.MethodPost, "/devices", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.Create(rec, r)

	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	key, _ := data["key"].(string)
	require.True(t, strings.HasPrefix(key, KeyScheme))
	device := data["device"].(map[string]any)
	require.Equal(t, "Booth 1", device["name"])
	require.Equal(t, eventID.String(), device["assignedEventId"])
	require.NotEqual(t, key, device["keyPrefix"])
}

func TestHandler_Create_ValidationAndNotFound(t *testing.T) {
	h, _, _ := newTestHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/devices", strings.NewReader(`{"name":""}`))
	rec := httptest.NewRecorder()
	h.Create(rec, r)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	r = httptest.NewRequest(http.MethodPost, "/devices", strings.NewReader(`{"name":"B","assignedEventId":"not-a-uuid"}`))
	rec = httptest.NewRecorder()
	h.Create(rec, r)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)

	r = httptest.NewRequest(http.MethodPost, "/devices", strings.NewReader(`{"name":"B","assignedEventId":"`+uuid.NewString()+`"}`))
	rec = httptest.NewRecorder()
	h.Create(rec, r)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")
}

func TestHandler_List(t *testing.T) {
	h, repo, userID := newTestHandler(t)
	active, _ := repo.seed(userID, "Booth 1", nil)
	repo.seed(uuid.New(), "Someone else", nil)
	require.NoError(t, h.svc.Revoke(context.Background(), userID, active.ID))

	r := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()
	h.List(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	items := data["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.Equal(t, "Booth 1", item["name"])
	require.NotNil(t, item["revokedAt"])
	require.Nil(t, item["lastUsedAt"])
}

func TestHandler_Update(t *testing.T) {
	h, repo, userID := newTestHandler(t)
	device, _ := repo.seed(userID, "Old", nil)

	r := withURLParams(httptest.NewRequest(http.MethodPatch, "/devices/x", strings.NewReader(`{"name":"New"}`)),
		map[string]string{"deviceID": device.ID.String()})
	rec := httptest.NewRecorder()
	h.Update(rec, r)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "New", decodeEnvelope(t, rec)["data"].(map[string]any)["name"])

	r = withURLParams(httptest.NewRequest(http.MethodPatch, "/devices/x", strings.NewReader(`{"name":"New"}`)),
		map[string]string{"deviceID": "not-a-uuid"})
	rec = httptest.NewRecorder()
	h.Update(rec, r)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Rotate(t *testing.T) {
	h, repo, userID := newTestHandler(t)
	device, oldRaw := repo.seed(userID, "Booth", nil)

	r := withURLParams(httptest.NewRequest(http.MethodPost, "/devices/x/rotate", nil),
		map[string]string{"deviceID": device.ID.String()})
	rec := httptest.NewRecorder()
	h.Rotate(rec, r)
	require.Equal(t, http.StatusOK, rec.Code)

	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	newRaw := data["key"].(string)
	require.NotEqual(t, oldRaw, newRaw)

	_, err := h.svc.Authenticate(context.Background(), oldRaw)
	require.Error(t, err)
	_, err = h.svc.Authenticate(context.Background(), newRaw)
	require.NoError(t, err)
}

func TestHandler_Revoke(t *testing.T) {
	h, repo, userID := newTestHandler(t)
	device, raw := repo.seed(userID, "Booth", nil)

	r := withURLParams(httptest.NewRequest(http.MethodDelete, "/devices/x", nil),
		map[string]string{"deviceID": device.ID.String()})
	rec := httptest.NewRecorder()
	h.Revoke(rec, r)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())

	_, err := h.svc.Authenticate(context.Background(), raw)
	require.Error(t, err)
	require.Equal(t, "DEVICE_REVOKED", appErrCode(t, err))
}

func TestHandler_Unauthenticated(t *testing.T) {
	svc, _, _ := newTestService(t)
	h := NewHandler(svc, func(*http.Request) (string, bool) { return "", false })

	r := httptest.NewRequest(http.MethodGet, "/devices", nil)
	rec := httptest.NewRecorder()
	h.List(rec, r)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "UNAUTHORIZED")
}
