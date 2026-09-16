package events

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

func newHandlerForTest(t *testing.T) (*Handler, uuid.UUID) {
	t.Helper()
	userID := uuid.New()
	svc := newTestService(newFakeRepo(), fixedLimits{})
	h := NewHandler(svc, func(r *http.Request) (string, bool) {
		return userID.String(), true
	})
	return h, userID
}

func serve(h http.HandlerFunc, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, r)
	return rec
}

func withURLParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) httpx.Envelope {
	t.Helper()
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return env
}

func TestHandler_Create(t *testing.T) {
	h, _ := newHandlerForTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events",
		strings.NewReader(`{"name":"Summer Party","clientEmail":"client@example.com"}`))
	rec := serve(h.Create, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec)
	if env.Error != nil {
		t.Fatalf("unexpected error %+v", env.Error)
	}
}

func TestHandler_CreateValidation(t *testing.T) {
	h, _ := newHandlerForTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":""}`))
	rec := serve(h.Create, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_CreateInvalidJSON(t *testing.T) {
	h, _ := newHandlerForTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{not json`))
	rec := serve(h.Create, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandler_List(t *testing.T) {
	h, _ := newHandlerForTest(t)
	_ = serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?status=upcoming&limit=10", nil)
	rec := serve(h.List, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_ListInvalidStatus(t *testing.T) {
	h, _ := newHandlerForTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events?status=bogus", nil)
	rec := serve(h.List, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandler_GetAndNotFound(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	created := env.Data.(map[string]any)["event"].(map[string]any)
	id := created["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+id, nil), "eventID", id)
	rec = serve(h.Get, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}

	missing := uuid.New().String()
	req = withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+missing, nil), "eventID", missing)
	rec = serve(h.Get, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "EVENT_NOT_FOUND") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_Update(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPatch, "/api/v1/events/"+id,
		strings.NewReader(`{"name":"Renamed"}`)), "eventID", id)
	rec = serve(h.Update, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Renamed") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_Archive(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/archive", nil), "eventID", id)
	rec = serve(h.Archive, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "archived") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UpdateStatus(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/status",
		strings.NewReader(`{"status":"active"}`)), "eventID", id)
	rec = serve(h.UpdateStatus, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"active"`) {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UpdateStatusInvalidTransition(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/status",
		strings.NewReader(`{"status":"completed"}`)), "eventID", id)
	rec = serve(h.UpdateStatus, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INVALID_STATUS_TRANSITION") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UpdateStatusValidation(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/status",
		strings.NewReader(`{"status":"bogus"}`)), "eventID", id)
	rec = serve(h.UpdateStatus, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_CreatePlanLimit(t *testing.T) {
	userID := uuid.New()
	svc := newTestService(newFakeRepo(), fixedLimits{max: 1})
	h := NewHandler(svc, func(r *http.Request) (string, bool) {
		return userID.String(), true
	})

	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events",
		strings.NewReader(`{"name":"First"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create first got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events",
		strings.NewReader(`{"name":"Second"}`)))
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "PLAN_LIMIT_REACHED") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UpdateStatusPlanLimit(t *testing.T) {
	userID := uuid.New()
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{max: 1})
	h := NewHandler(svc, func(r *http.Request) (string, bool) {
		return userID.String(), true
	})

	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events",
		strings.NewReader(`{"name":"First"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create first got %d body=%s", rec.Code, rec.Body.String())
	}

	// An expired event occupies no slot, so reactivating it is what trips the
	// limit when a live event already exists.
	expiredID := uuid.New()
	repo.events[expiredID] = &Event{ID: expiredID, UserID: userID, Name: "Expired", Status: StatusExpired}
	repo.settings[expiredID] = &Settings{EventID: expiredID}
	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+expiredID.String()+"/status",
		strings.NewReader(`{"status":"active"}`)), "eventID", expiredID.String())
	rec = serve(h.UpdateStatus, req)
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "PLAN_LIMIT_REACHED") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_DeleteNoContent(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodDelete, "/api/v1/events/"+id, nil), "eventID", id)
	rec = serve(h.Delete, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %s", rec.Body.String())
	}
}

func TestHandler_GetSettings(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+id+"/settings", nil), "eventID", id)
	rec = serve(h.GetSettings, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "public") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UpdateSettings(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPatch, "/api/v1/events/"+id+"/settings",
		strings.NewReader(`{"visibility":"password","password":"secret"}`)), "eventID", id)
	rec = serve(h.UpdateSettings, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_Dashboard(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodGet, "/api/v1/events/"+id+"/dashboard", nil), "eventID", id)
	rec = serve(h.Dashboard, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "photoCount") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_UnauthorizedWhenNoUser(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	h := NewHandler(svc, func(r *http.Request) (string, bool) { return "", false })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rec := serve(h.List, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandler_Extend(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/extend",
		strings.NewReader(`{"days":30}`)), "eventID", id)
	rec = serve(h.Extend, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "expiresAt") {
		t.Fatalf("body %s", rec.Body.String())
	}
}

func TestHandler_ExtendValidation(t *testing.T) {
	h, _ := newHandlerForTest(t)
	rec := serve(h.Create, httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(`{"name":"Party"}`)))
	env := decodeEnvelope(t, rec)
	id := env.Data.(map[string]any)["event"].(map[string]any)["id"].(string)

	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+id+"/extend",
		strings.NewReader(`{"days":0}`)), "eventID", id)
	rec = serve(h.Extend, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_ExtendNotFound(t *testing.T) {
	h, _ := newHandlerForTest(t)
	missing := uuid.New().String()
	req := withURLParam(httptest.NewRequest(http.MethodPost, "/api/v1/events/"+missing+"/extend",
		strings.NewReader(`{"days":30}`)), "eventID", missing)
	rec := serve(h.Extend, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}
