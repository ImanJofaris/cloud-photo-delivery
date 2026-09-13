package exports

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
	"github.com/stretchr/testify/require"
)

func handlerForTest(t *testing.T, userID uuid.UUID, svc *Service) *Handler {
	t.Helper()
	return NewHandler(svc, func(*http.Request) (string, bool) {
		return userID.String(), true
	})
}

func withURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHandler_CreateAccepted(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	h := handlerForTest(t, uuid.New(), svc)
	eventID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID.String()+"/exports", nil),
		map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())
	var env struct {
		Data  map[string]any `json:"data"`
		Error any            `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Nil(t, env.Error)
	require.Equal(t, "pending", env.Data["status"])
	require.Equal(t, eventID.String(), env.Data["eventId"])
	require.Nil(t, env.Data["downloadUrl"])
}

func TestHandler_CreateValidationError(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeEvents{owned: true}, fakeCounter{count: 0}, &fakeQueue{}, &fakePresigner{}, time.Hour)
	h := handlerForTest(t, uuid.New(), svc)
	eventID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID.String()+"/exports", nil),
		map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), "VALIDATION_ERROR")
}

func TestHandler_CreateEventNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, fakeEvents{owned: false}, fakeCounter{count: 3}, &fakeQueue{}, &fakePresigner{}, time.Hour)
	h := handlerForTest(t, uuid.New(), svc)
	eventID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID.String()+"/exports", nil),
		map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")
}

func TestHandler_CreateBadUUID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	h := handlerForTest(t, uuid.New(), svc)

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/events/not-a-uuid/exports", nil),
		map[string]string{"eventID": "not-a-uuid"})
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_CreateUnauthorized(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	h := NewHandler(svc, func(*http.Request) (string, bool) { return "", false })
	eventID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID.String()+"/exports", nil),
		map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_GetReadyIncludesDownloadURL(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	userID := uuid.New()
	eventID := uuid.New()
	key := "tenant/x/exports/e.zip"
	expires := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
	export := repo.seed(&Export{
		ID: uuid.New(), EventID: eventID, Status: StatusReady, ObjectKey: &key, ExpiresAt: &expires,
	})
	h := handlerForTest(t, userID, svc)

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/exports/"+export.ID.String(), nil),
		map[string]string{"eventID": eventID.String(), "exportID": export.ID.String()})
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var env struct {
		Data struct {
			ID          string  `json:"id"`
			Status      string  `json:"status"`
			DownloadURL *string `json:"downloadUrl"`
			CreatedAt   string  `json:"createdAt"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, export.ID.String(), env.Data.ID)
	require.Equal(t, "ready", env.Data.Status)
	require.NotNil(t, env.Data.DownloadURL)
	require.NotEmpty(t, env.Data.CreatedAt)
}

func TestHandler_GetNotFound(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	h := handlerForTest(t, uuid.New(), svc)
	eventID := uuid.New()
	exportID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/exports/"+exportID.String(), nil),
		map[string]string{"eventID": eventID.String(), "exportID": exportID.String()})
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EXPORT_NOT_FOUND")
}

func TestHandler_GetBadExportUUID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	h := handlerForTest(t, uuid.New(), svc)
	eventID := uuid.New()

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+eventID.String()+"/exports/nope", nil),
		map[string]string{"eventID": eventID.String(), "exportID": "nope"})
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EXPORT_NOT_FOUND")
}

func TestHandler_EnvelopeShape(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	eventID := uuid.New()
	export := repo.seed(&Export{ID: uuid.New(), EventID: eventID, Status: StatusPending})
	h := handlerForTest(t, uuid.New(), svc)

	req := withURLParams(httptest.NewRequest(http.MethodGet, "/x", nil),
		map[string]string{"eventID": eventID.String(), "exportID": export.ID.String()})
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	var env httpx.Envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Nil(t, env.Error)
	require.Contains(t, rec.Body.String(), `"data"`)
	require.Contains(t, rec.Body.String(), `"error":null`)
}
