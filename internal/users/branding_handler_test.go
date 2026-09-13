package users

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func newTestBrandingHandler(repo *fakeBrandingRepo, store *fakeAssetStore, userID uuid.UUID) *BrandingHandler {
	svc := newBrandingService(repo, store)
	return NewBrandingHandler(svc, func(*http.Request) (string, bool) {
		return userID.String(), true
	})
}

func decodeBrandingEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env
}

func TestBrandingHandler_Get(t *testing.T) {
	userID := uuid.New()
	repo := &fakeBrandingRepo{branding: &Branding{
		UserID:       userID,
		BusinessName: "Booth Co",
		PrimaryColor: "#112233",
	}}
	h := newTestBrandingHandler(repo, &fakeAssetStore{}, userID)

	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/account/branding", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeBrandingEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, "Booth Co", data["businessName"])
	require.Equal(t, "#112233", data["primaryColor"])
	require.Nil(t, data["logoUrl"])
	require.Nil(t, data["websiteUrl"])
}

func TestBrandingHandler_Get_Empty(t *testing.T) {
	userID := uuid.New()
	h := newTestBrandingHandler(&fakeBrandingRepo{}, &fakeAssetStore{}, userID)

	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/account/branding", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	data := decodeBrandingEnvelope(t, rec)["data"].(map[string]any)
	for _, field := range []string{"businessName", "logoUrl", "profileImageUrl", "primaryColor", "secondaryColor", "contactEmail", "contactPhone", "websiteUrl"} {
		require.Nil(t, data[field], field)
	}
}

func TestBrandingHandler_Update(t *testing.T) {
	userID := uuid.New()
	repo := &fakeBrandingRepo{}
	h := newTestBrandingHandler(repo, &fakeAssetStore{}, userID)

	body := `{"businessName":"Booth Co","primaryColor":"#AABBCC","contactEmail":"hi@example.com"}`
	rec := httptest.NewRecorder()
	h.Update(rec, httptest.NewRequest(http.MethodPatch, "/account/branding", strings.NewReader(body)))

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeBrandingEnvelope(t, rec)["data"].(map[string]any)
	require.Equal(t, "Booth Co", data["businessName"])
	require.Equal(t, "#aabbcc", data["primaryColor"])
	require.Equal(t, "hi@example.com", data["contactEmail"])
}

func TestBrandingHandler_Update_Invalid(t *testing.T) {
	h := newTestBrandingHandler(&fakeBrandingRepo{}, &fakeAssetStore{}, uuid.New())

	rec := httptest.NewRecorder()
	h.Update(rec, httptest.NewRequest(http.MethodPatch, "/account/branding", strings.NewReader(`{"primaryColor":"blue"}`)))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "VALIDATION_ERROR", decodeBrandingEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestBrandingHandler_Update_UnknownField(t *testing.T) {
	h := newTestBrandingHandler(&fakeBrandingRepo{}, &fakeAssetStore{}, uuid.New())

	rec := httptest.NewRecorder()
	h.Update(rec, httptest.NewRequest(http.MethodPatch, "/account/branding", strings.NewReader(`{"nope":1}`)))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestBrandingHandler_CreateAssetUpload(t *testing.T) {
	userID := uuid.New()
	h := newTestBrandingHandler(&fakeBrandingRepo{}, &fakeAssetStore{}, userID)

	rec := httptest.NewRecorder()
	h.CreateAssetUpload(rec, httptest.NewRequest(http.MethodPost, "/account/branding/assets",
		strings.NewReader(`{"kind":"logo","contentType":"image/png"}`)))

	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	data := decodeBrandingEnvelope(t, rec)["data"].(map[string]any)
	require.Contains(t, data["uploadUrl"], "tenant/"+userID.String()+"/branding/logo/")
	require.Contains(t, data["storageKey"], "tenant/"+userID.String()+"/branding/logo/")
	require.NotEmpty(t, data["expiresAt"])
}

func TestBrandingHandler_CreateAssetUpload_InvalidKind(t *testing.T) {
	h := newTestBrandingHandler(&fakeBrandingRepo{}, &fakeAssetStore{}, uuid.New())

	rec := httptest.NewRecorder()
	h.CreateAssetUpload(rec, httptest.NewRequest(http.MethodPost, "/account/branding/assets",
		strings.NewReader(`{"kind":"avatar","contentType":"image/png"}`)))

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestBrandingHandler_Unauthenticated(t *testing.T) {
	svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{})
	h := NewBrandingHandler(svc, func(*http.Request) (string, bool) { return "", false })

	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/account/branding", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
