package billing

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/stretchr/testify/require"
)

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func newTestHandler(t *testing.T) (*Handler, *fakeRepo, *provider.Manual) {
	t.Helper()
	repo := newFakeRepo()
	seedPlans(repo)
	manual := provider.NewManual("test-secret")
	clock := func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) }
	manual.SetClock(clock)
	svc := NewService(repo, manual)
	svc.SetClock(clock)
	userID := uuid.New()
	h := NewHandler(svc, manual, func(*http.Request) (string, bool) {
		return userID.String(), true
	})
	return h, repo, manual
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	return env
}

func doHandler(t *testing.T, fn http.HandlerFunc, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Buffer
	if body == "" {
		rdr = bytes.NewBuffer(nil)
	} else {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	fn(rec, req)
	return rec
}

func TestHandler_PlansEnvelope(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Plans, http.MethodGet, "/api/v1/billing/plans", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"items"`)
	require.Contains(t, rec.Body.String(), `"free"`)
	require.Contains(t, rec.Body.String(), `"business"`)
}

func TestHandler_GetSubscriptionShowsFreePlan(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.GetSubscription, http.MethodGet, "/api/v1/billing/subscription", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	env := decodeEnvelope(t, rec)
	require.Nil(t, env.Error)
	var data struct {
		Subscription any `json:"subscription"`
		Plan         struct {
			ID string `json:"id"`
		} `json:"plan"`
		Usage struct {
			ActiveEvents int `json:"activeEvents"`
		} `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	require.Nil(t, data.Subscription)
	require.Equal(t, "free", data.Plan.ID)
}

func TestHandler_SubscribeReturnsCheckout(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())

	env := decodeEnvelope(t, rec)
	var data struct {
		Subscription struct {
			Status      string `json:"status"`
			ProviderRef string `json:"providerRef"`
		} `json:"subscription"`
		Checkout struct {
			URL string `json:"url"`
		} `json:"checkout"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	require.Equal(t, "active", data.Subscription.Status)
	require.NotEmpty(t, data.Subscription.ProviderRef)
	require.Contains(t, data.Checkout.URL, "manual://checkout/")
}

func TestHandler_SubscribeUnknownPlanIs404(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"nope"}`, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "PLAN_NOT_FOUND", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_SubscribeInvalidBodyIs422(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":`, nil)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	require.Equal(t, "VALIDATION_ERROR", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_UpgradeCancelResume(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = doHandler(t, h.Upgrade, http.MethodPost, "/api/v1/billing/upgrade", `{"planId":"pro"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"pro"`)

	rec = doHandler(t, h.Cancel, http.MethodPost, "/api/v1/billing/cancel", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"cancelAtPeriodEnd":true`)

	rec = doHandler(t, h.Resume, http.MethodPost, "/api/v1/billing/resume", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"cancelAtPeriodEnd":false`)
}

func TestHandler_InvoicesEnvelope(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = doHandler(t, h.Invoices, http.MethodGet, "/api/v1/billing/invoices", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	env := decodeEnvelope(t, rec)
	var data struct {
		Items      []map[string]any `json:"items"`
		NextCursor any              `json:"nextCursor"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	require.Len(t, data.Items, 1)
	require.Equal(t, "open", data.Items[0]["status"])
}

func TestHandler_WebhookRejectsInvalidSignature(t *testing.T) {
	h, _, _ := newTestHandler(t)
	body := `{"id":"evt_1","type":"invoice.paid"}`
	rec := doHandler(t, h.Webhook, http.MethodPost, "/api/v1/billing/webhook", body, nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "INVALID_WEBHOOK_SIGNATURE", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_WebhookAppliesValidEvent(t *testing.T) {
	h, repo, manual := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	env := decodeEnvelope(t, rec)
	var data struct {
		Subscription struct {
			ProviderRef string `json:"providerRef"`
		} `json:"subscription"`
	}
	require.NoError(t, json.Unmarshal(env.Data, &data))
	require.NotEmpty(t, data.Subscription.ProviderRef)

	body := `{"id":"evt_paid_1","type":"invoice.paid","providerRef":"` + data.Subscription.ProviderRef + `"}`
	rec = doHandler(t, h.Webhook, http.MethodPost, "/api/v1/billing/webhook", body,
		map[string]string{provider.SignatureHeader: manual.Sign([]byte(body))})
	require.Equal(t, http.StatusAccepted, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, InvoicePaid, repo.invoices[0].Status)

	rec = doHandler(t, h.Webhook, http.MethodPost, "/api/v1/billing/webhook", body,
		map[string]string{provider.SignatureHeader: manual.Sign([]byte(body))})
	require.Equal(t, http.StatusAccepted, rec.Code, "duplicates are acknowledged")
	require.Equal(t, 1, repo.markPaidCalls)
}

func TestHandler_UnauthorizedWithoutActor(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	manual := provider.NewManual("test-secret")
	svc := NewService(repo, manual)
	h := NewHandler(svc, manual, func(*http.Request) (string, bool) { return "", false })

	rec := doHandler(t, h.Plans, http.MethodGet, "/api/v1/billing/plans", "", nil)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "UNAUTHORIZED", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_DowngradeAndMissingSubscriptionErrors(t *testing.T) {
	h, _, _ := newTestHandler(t)

	rec := doHandler(t, h.Upgrade, http.MethodPost, "/api/v1/billing/upgrade", `{"planId":"pro"}`, nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	rec = doHandler(t, h.Cancel, http.MethodPost, "/api/v1/billing/cancel", "", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	rec = doHandler(t, h.Resume, http.MethodPost, "/api/v1/billing/resume", "", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = doHandler(t, h.Downgrade, http.MethodPost, "/api/v1/billing/downgrade", `{"planId":"free"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Contains(t, rec.Body.String(), `"planId":"free"`)

	rec = doHandler(t, h.Upgrade, http.MethodPost, "/api/v1/billing/upgrade", `{"planId":`, nil)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandler_GetSubscriptionWithActivePlan(t *testing.T) {
	h, _, _ := newTestHandler(t)
	rec := doHandler(t, h.Subscribe, http.MethodPost, "/api/v1/billing/subscribe", `{"planId":"starter"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = doHandler(t, h.GetSubscription, http.MethodGet, "/api/v1/billing/subscription", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"subscription":{`)
	require.Contains(t, rec.Body.String(), `"planId":"starter"`)
}

func TestHandler_PlansServiceError(t *testing.T) {
	h, repo, _ := newTestHandler(t)
	repo.listPlansErr = errors.New("db down")

	rec := doHandler(t, h.Plans, http.MethodGet, "/api/v1/billing/plans", "", nil)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "INTERNAL_ERROR", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_WebhookMalformedPayload(t *testing.T) {
	h, _, manual := newTestHandler(t)
	body := `{"id":`
	rec := doHandler(t, h.Webhook, http.MethodPost, "/api/v1/billing/webhook", body,
		map[string]string{provider.SignatureHeader: manual.Sign([]byte(body))})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "INVALID_WEBHOOK_PAYLOAD", decodeEnvelope(t, rec).Error.Code)
}

func TestHandler_ParseLimit(t *testing.T) {
	require.Equal(t, 0, parseLimit(""))
	require.Equal(t, 7, parseLimit("7"))
	require.Equal(t, 0, parseLimit("abc"))
	require.Equal(t, 10000, parseLimit("999999"))
}
