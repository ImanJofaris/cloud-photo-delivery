package gallery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/stretchr/testify/require"
)

func withParams(r *http.Request, params map[string]string) *http.Request {
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

func newTestHandler() (*Handler, *fakeRepo) {
	svc, repo, _ := newTestService()
	return NewHandler(svc), repo
}

func TestHandler_GetEvent_PublicCacheHeader(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding", nil), map[string]string{"slug": "wedding"})
	rec := httptest.NewRecorder()
	h.GetEvent(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, cachePublic, rec.Header().Get("Cache-Control"))
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, "Wedding", data["name"])
	require.Equal(t, false, data["requiresUnlock"])
}

func TestHandler_GetEvent_PasswordCacheHeader(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:x"
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/pw", nil), map[string]string{"slug": "pw"})
	rec := httptest.NewRecorder()
	h.GetEvent(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, cachePrivate, rec.Header().Get("Cache-Control"))
	require.True(t, decodeEnvelope(t, rec)["data"].(map[string]any)["requiresUnlock"].(bool))
}

func TestHandler_GetEvent_PrivateReturns404(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("secret")
	s.Visibility = events.VisibilityPrivate
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/secret", nil), map[string]string{"slug": "secret"})
	rec := httptest.NewRecorder()
	h.GetEvent(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Unlock_SuccessNoStore(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodPost, "/public/events/pw/unlock", strings.NewReader(`{"password":"right"}`)), map[string]string{"slug": "pw"})
	rec := httptest.NewRecorder()
	h.Unlock(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, cachePrivate, rec.Header().Get("Cache-Control"))
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.NotEmpty(t, data["token"])
}

func TestHandler_ListPhotos_MetadataOnly(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	base := time.Now().UTC()
	repo.photos[e.ID] = append(repo.photos[e.ID], readyPhoto(e.ID, base))

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding/photos", nil), map[string]string{"slug": "wedding"})
	rec := httptest.NewRecorder()
	h.ListPhotos(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.NotContains(t, body, "http://")
	require.NotContains(t, body, "thumbnailUrl")
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	photosObj := data["photos"].(map[string]any)
	items := photosObj["items"].([]any)
	require.Len(t, items, 1)
	item := items[0].(map[string]any)
	require.NotContains(t, item, "thumbnailUrl")
	require.Contains(t, item, "variants")
}

func TestHandler_PhotoURL_PrivateCacheHeader(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	p := readyPhoto(e.ID, time.Now())
	repo.photos[e.ID] = append(repo.photos[e.ID], p)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding/photos/"+p.ID.String()+"/url?variant=large", nil), map[string]string{"slug": "wedding", "photoID": p.ID.String()})
	rec := httptest.NewRecorder()
	h.PhotoURL(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, cachePrivate, rec.Header().Get("Cache-Control"))
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.NotEmpty(t, data["url"])
	require.Equal(t, float64(300), data["expiresIn"])
}

func TestHandler_PhotoURL_InvalidVariant422(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding/photos/"+e.ID.String()+"/url?variant=bogus", nil), map[string]string{"slug": "wedding", "photoID": e.ID.String()})
	rec := httptest.NewRecorder()
	h.PhotoURL(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandler_GetPhoto_MetadataOnly(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	p := readyPhoto(e.ID, time.Now())
	repo.photos[e.ID] = append(repo.photos[e.ID], p)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding/photos/"+p.ID.String(), nil), map[string]string{"slug": "wedding", "photoID": p.ID.String()})
	rec := httptest.NewRecorder()
	h.GetPhoto(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "http://")
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, p.ID.String(), data["id"])
}

func TestHandler_ListPhotos_PasswordWithoutToken401(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/pw/photos", nil), map[string]string{"slug": "pw"})
	rec := httptest.NewRecorder()
	h.ListPhotos(rec, r)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestHandler_Unlock_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler()
	r := withParams(httptest.NewRequest(http.MethodPost, "/public/events/pw/unlock", strings.NewReader("{bad")), map[string]string{"slug": "pw"})
	rec := httptest.NewRecorder()
	h.Unlock(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "VALIDATION_ERROR", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_GetEvent_Branding(t *testing.T) {
	svc, repo, _, branding := newTestServiceFull()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	branding.view = &users.BrandingView{
		BusinessName: "Booth Co",
		PrimaryColor: "#112233",
		ContactEmail: "hello@example.com",
	}
	h := NewHandler(svc)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding", nil), map[string]string{"slug": "wedding"})
	rec := httptest.NewRecorder()
	h.GetEvent(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	b := data["branding"].(map[string]any)
	require.Equal(t, "Booth Co", b["businessName"])
	require.Equal(t, "#112233", b["primaryColor"])
	require.Equal(t, "hello@example.com", b["contactEmail"])
	require.Nil(t, b["logoUrl"])
}

func TestHandler_GetEvent_BrandingNullWhenUnset(t *testing.T) {
	h, repo := newTestHandler()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding", nil), map[string]string{"slug": "wedding"})
	rec := httptest.NewRecorder()
	h.GetEvent(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Nil(t, data["branding"])
}

func TestHandler_ListPhotos_IncludesBranding(t *testing.T) {
	svc, repo, _, branding := newTestServiceFull()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	branding.view = &users.BrandingView{BusinessName: "Booth Co"}
	h := NewHandler(svc)

	r := withParams(httptest.NewRequest(http.MethodGet, "/public/events/wedding/photos", nil), map[string]string{"slug": "wedding"})
	rec := httptest.NewRecorder()
	h.ListPhotos(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	event := data["event"].(map[string]any)
	require.Equal(t, "Booth Co", event["branding"].(map[string]any)["businessName"])
}
