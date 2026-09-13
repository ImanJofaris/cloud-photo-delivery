package qr

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

func withParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func newTestHandler() (*Handler, *fakeEventLookup) {
	svc, lookup := newTestService()
	h := NewHandler(svc, func(*http.Request) (string, bool) {
		return lookup.userID.String(), true
	})
	return h, lookup
}

func TestHandler_PNG(t *testing.T) {
	h, lookup := newTestHandler()
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/"+lookup.event.ID.String()+"/qr.png?size=256", nil),
		map[string]string{"eventID": lookup.event.ID.String()})
	rec := httptest.NewRecorder()
	h.PNG(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, PNGContentType, rec.Header().Get("Content-Type"))
	require.Equal(t, cacheQR, rec.Header().Get("Cache-Control"))
	require.Equal(t, "https://photos.example.com/e/iman-wedding", decodePNG(t, rec.Body.Bytes()))
}

func TestHandler_SVG(t *testing.T) {
	h, lookup := newTestHandler()
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/"+lookup.event.ID.String()+"/qr.svg", nil),
		map[string]string{"eventID": lookup.event.ID.String()})
	rec := httptest.NewRecorder()
	h.SVG(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, SVGContentType, rec.Header().Get("Content-Type"))
	require.Equal(t, cacheQR, rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Body.String(), "<svg")
}

func TestHandler_URL(t *testing.T) {
	h, lookup := newTestHandler()
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/"+lookup.event.ID.String()+"/url", nil),
		map[string]string{"eventID": lookup.event.ID.String()})
	rec := httptest.NewRecorder()
	h.URL(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, cacheURL, rec.Header().Get("Cache-Control"))
	var env struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, "https://photos.example.com/e/iman-wedding", env.Data.URL)
}

func TestHandler_OtherTenantGets404(t *testing.T) {
	svc, lookup := newTestService()
	h := NewHandler(svc, func(*http.Request) (string, bool) {
		return uuid.NewString(), true
	})
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/"+lookup.event.ID.String()+"/qr.png", nil),
		map[string]string{"eventID": lookup.event.ID.String()})
	rec := httptest.NewRecorder()
	h.PNG(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Contains(t, rec.Body.String(), "EVENT_NOT_FOUND")
}

func TestHandler_Unauthenticated401(t *testing.T) {
	h, lookup := newTestHandler()
	h.getID = func(*http.Request) (string, bool) { return "", false }
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/"+lookup.event.ID.String()+"/qr.svg", nil),
		map[string]string{"eventID": lookup.event.ID.String()})
	rec := httptest.NewRecorder()
	h.SVG(rec, r)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_InvalidEventID404(t *testing.T) {
	h, _ := newTestHandler()
	r := withParams(httptest.NewRequest(http.MethodGet, "/events/nope/qr.svg", nil), map[string]string{"eventID": "nope"})
	rec := httptest.NewRecorder()
	h.SVG(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestParseSize(t *testing.T) {
	require.Equal(t, 0, parseSize(""))
	require.Equal(t, 0, parseSize("abc"))
	require.Equal(t, 256, parseSize("256"))
	require.Equal(t, 100000, parseSize("999999"))
}
