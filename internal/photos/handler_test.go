package photos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

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

func newTestHandler() (*Handler, uuid.UUID, uuid.UUID, *Photo, *cleanupQueueFake) {
	repo, userID, eventID, photo := ownerFixture()
	repo.listed = []*Photo{photo}
	repo.deleteResult = &DeletedPhoto{EventID: eventID, Keys: []string{photo.StorageKey}}
	svc, _, _, queue := newTestService(repo)
	h := NewHandler(svc, func(*http.Request) (Actor, bool) { return Actor{UserID: userID}, true })
	return h, userID, eventID, photo, queue
}

func TestHandler_List_ReturnsEnvelope(t *testing.T) {
	h, _, eventID, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/events/x/photos", nil), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.List(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	require.Len(t, data["items"].([]any), 1)
	require.Nil(t, data["nextCursor"])
	item := data["items"].([]any)[0].(map[string]any)
	require.Equal(t, "READY", item["status"])
	require.Len(t, item["variants"].([]any), 3)
}

func TestHandler_List_InvalidEventID(t *testing.T) {
	h, _, _, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/events/x/photos", nil), map[string]string{"eventID": "not-a-uuid"})
	rec := httptest.NewRecorder()
	h.List(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "EVENT_NOT_FOUND", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_List_InvalidCursor(t *testing.T) {
	h, _, eventID, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/events/x/photos?cursor=%21%21", nil), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.List(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "VALIDATION_ERROR", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_List_Unauthenticated(t *testing.T) {
	repo, _, eventID, _ := ownerFixture()
	svc, _, _, _ := newTestService(repo)
	h := NewHandler(svc, func(*http.Request) (Actor, bool) { return Actor{}, false })

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/events/x/photos", nil), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.List(rec, r)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_URL_ReturnsSignedURL(t *testing.T) {
	h, _, _, photo, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/photos/x/url?variant=thumbnail", nil), map[string]string{"photoID": photo.ID.String()})
	rec := httptest.NewRecorder()
	h.URL(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.NotEmpty(t, data["url"])
	require.EqualValues(t, 300, data["expiresIn"])
}

func TestHandler_URL_InvalidVariant(t *testing.T) {
	h, _, _, photo, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/photos/x/url?variant=huge", nil), map[string]string{"photoID": photo.ID.String()})
	rec := httptest.NewRecorder()
	h.URL(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandler_URL_UnknownPhotoIsNotFound(t *testing.T) {
	h, _, _, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/photos/x/url?variant=thumbnail", nil), map[string]string{"photoID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.URL(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "PHOTO_NOT_FOUND", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_Delete_NoContentAndEnqueues(t *testing.T) {
	h, _, _, photo, queue := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodDelete, "/photos/x", nil), map[string]string{"photoID": photo.ID.String()})
	rec := httptest.NewRecorder()
	h.Delete(rec, r)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
	require.Len(t, queue.payloads, 1)
}

func TestHandler_Delete_ForeignPhotoNotFound(t *testing.T) {
	repo, _, _, photo := ownerFixture()
	svc, _, _, _ := newTestService(repo)
	h := NewHandler(svc, func(*http.Request) (Actor, bool) { return Actor{UserID: uuid.New()}, true })

	r := withURLParams(httptest.NewRequest(http.MethodDelete, "/photos/x", nil), map[string]string{"photoID": photo.ID.String()})
	rec := httptest.NewRecorder()
	h.Delete(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "PHOTO_NOT_FOUND", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}
