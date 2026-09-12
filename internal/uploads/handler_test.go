package uploads

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

func newTestHandler() (*Handler, uuid.UUID, uuid.UUID, *fakePhotoRepo) {
	photoRepo := newFakePhotoRepo()
	uploadRepo := newFakeUploadRepo()
	store := &fakeStore{headSize: 1024}
	queue := &fakeQueue{}
	svc := NewService(photoRepo, uploadRepo, store, queue)

	userID := uuid.New()
	eventID := uuid.New()
	uploadRepo.owned[eventID] = true
	uploadRepo.owners[eventID] = userID

	h := NewHandler(svc, func(*http.Request) (string, bool) { return userID.String(), true })
	return h, userID, eventID, photoRepo
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

func TestHandler_Initialize_ReturnsCreated(t *testing.T) {
	h, _, eventID, _ := newTestHandler()

	body := `{"filename":"a.jpg","contentType":"image/jpeg","size":1024}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/"+eventID.String()+"/uploads", strings.NewReader(body)), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)

	require.Equal(t, http.StatusCreated, rec.Code)
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	require.NotEmpty(t, data["photoId"])
	require.Equal(t, "simple", data["uploadKind"])
}

func TestHandler_Initialize_ValidationError(t *testing.T) {
	h, _, eventID, _ := newTestHandler()

	body := `{"filename":"a.gif","contentType":"image/gif","size":1024}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/"+eventID.String()+"/uploads", strings.NewReader(body)), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	env := decodeEnvelope(t, rec)
	errBody := env["error"].(map[string]any)
	require.Equal(t, "VALIDATION_ERROR", errBody["code"])
}

func TestHandler_Initialize_InvalidJSON(t *testing.T) {
	h, _, eventID, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/"+eventID.String()+"/uploads", strings.NewReader("{bad")), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandler_Status_NotFoundForUnknownPhoto(t *testing.T) {
	h, _, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/uploads/x", nil), map[string]string{"photoID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.Status(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	env := decodeEnvelope(t, rec)
	errBody := env["error"].(map[string]any)
	require.Equal(t, "UPLOAD_NOT_FOUND", errBody["code"])
}

func TestHandler_Status_InvalidUUIDIsNotFound(t *testing.T) {
	h, _, _, _ := newTestHandler()

	r := withURLParams(httptest.NewRequest(http.MethodGet, "/uploads/x", nil), map[string]string{"photoID": "not-a-uuid"})
	rec := httptest.NewRecorder()
	h.Status(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandler_Complete_Envelope(t *testing.T) {
	h, _, eventID, _ := newTestHandler()

	initBody := `{"filename":"a.jpg","contentType":"image/jpeg","size":1024}`
	initReq := withURLParams(httptest.NewRequest(http.MethodPost, "/events/x/uploads", strings.NewReader(initBody)), map[string]string{"eventID": eventID.String()})
	initRec := httptest.NewRecorder()
	h.Initialize(initRec, initReq)
	require.Equal(t, http.StatusCreated, initRec.Code)
	photoID := decodeEnvelope(t, initRec)["data"].(map[string]any)["photoId"].(string)

	r := withURLParams(httptest.NewRequest(http.MethodPost, "/uploads/x/complete", nil), map[string]string{"photoID": photoID})
	rec := httptest.NewRecorder()
	h.Complete(rec, r)

	require.Equal(t, http.StatusOK, rec.Code)
	env := decodeEnvelope(t, rec)
	data := env["data"].(map[string]any)
	require.Equal(t, "PROCESSING", data["status"])
}

func TestHandler_Unauthenticated(t *testing.T) {
	photoRepo := newFakePhotoRepo()
	uploadRepo := newFakeUploadRepo()
	svc := NewService(photoRepo, uploadRepo, &fakeStore{}, &fakeQueue{})
	h := NewHandler(svc, func(*http.Request) (string, bool) { return "", false })

	r := httptest.NewRequest(http.MethodGet, "/uploads/x", nil)
	rec := httptest.NewRecorder()
	h.Status(rec, r)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func initMultipart(t *testing.T, h *Handler, eventID uuid.UUID) string {
	t.Helper()
	body := `{"filename":"big.jpg","contentType":"image/jpeg","size":20971520}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/x/uploads", strings.NewReader(body)), map[string]string{"eventID": eventID.String()})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	return decodeEnvelope(t, rec)["data"].(map[string]any)["photoId"].(string)
}

func TestHandler_Parts_ReturnsURLs(t *testing.T) {
	h, _, eventID, _ := newTestHandler()
	photoID := initMultipart(t, h, eventID)

	body := `{"partNumbers":[1,2]}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/uploads/x/parts", strings.NewReader(body)), map[string]string{"photoID": photoID})
	rec := httptest.NewRecorder()
	h.Parts(rec, r)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := decodeEnvelope(t, rec)["data"].(map[string]any)
	require.Len(t, data["parts"].([]any), 2)
}

func TestHandler_Parts_NotMultipartConflict(t *testing.T) {
	h, _, eventID, _ := newTestHandler()

	initBody := `{"filename":"a.jpg","contentType":"image/jpeg","size":1024}`
	initReq := withURLParams(httptest.NewRequest(http.MethodPost, "/events/x/uploads", strings.NewReader(initBody)), map[string]string{"eventID": eventID.String()})
	initRec := httptest.NewRecorder()
	h.Initialize(initRec, initReq)
	photoID := decodeEnvelope(t, initRec)["data"].(map[string]any)["photoId"].(string)

	body := `{"partNumbers":[1]}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/uploads/x/parts", strings.NewReader(body)), map[string]string{"photoID": photoID})
	rec := httptest.NewRecorder()
	h.Parts(rec, r)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Equal(t, "NOT_MULTIPART", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_CompleteMultipart(t *testing.T) {
	h, _, eventID, photoRepo := newTestHandler()
	photoID := initMultipart(t, h, eventID)

	body := `{"parts":[{"partNumber":1,"etag":"etag-1"}]}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/uploads/x/multipart/complete", strings.NewReader(body)), map[string]string{"photoID": photoID})
	rec := httptest.NewRecorder()
	h.CompleteMultipart(rec, r)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, "etag-1", photoRepo.parts[uuid.MustParse(photoID)][1])
}

func TestHandler_AbortMultipart(t *testing.T) {
	h, _, eventID, _ := newTestHandler()
	photoID := initMultipart(t, h, eventID)

	r := withURLParams(httptest.NewRequest(http.MethodPost, "/uploads/x/multipart/abort", nil), map[string]string{"photoID": photoID})
	rec := httptest.NewRecorder()
	h.AbortMultipart(rec, r)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}

func TestHandler_Initialize_RejectsUnknownEvent(t *testing.T) {
	h, _, _, _ := newTestHandler()
	body := `{"filename":"a.jpg","contentType":"image/jpeg","size":1024}`
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/x/uploads", strings.NewReader(body)), map[string]string{"eventID": uuid.New().String()})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "EVENT_NOT_FOUND", decodeEnvelope(t, rec)["error"].(map[string]any)["code"])
}

func TestHandler_Initialize_InvalidEventID(t *testing.T) {
	h, _, _, _ := newTestHandler()
	r := withURLParams(httptest.NewRequest(http.MethodPost, "/events/x/uploads", strings.NewReader("{}")), map[string]string{"eventID": "not-a-uuid"})
	rec := httptest.NewRecorder()
	h.Initialize(rec, r)

	require.Equal(t, http.StatusNotFound, rec.Code)
}
