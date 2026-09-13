package provider

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testManual() *Manual {
	m := NewManual("test-secret")
	m.SetClock(func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) })
	return m
}

func webhookRequest(body string, header string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/billing/webhook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if header != "" {
		req.Header.Set(SignatureHeader, header)
	}
	return req
}

func TestManual_CreateCheckoutIsOffline(t *testing.T) {
	m := testManual()
	session, err := m.CreateCheckout(context.Background(), Subscription{ID: "sub-1"})
	require.NoError(t, err)
	require.Contains(t, session.ProviderRef, "manual_checkout_")
	require.Equal(t, "manual://checkout/"+session.ProviderRef, session.URL)
	require.Equal(t, time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC), session.ExpiresAt)
}

func TestManual_CancelIsNoOp(t *testing.T) {
	m := testManual()
	require.NoError(t, m.Cancel(context.Background(), "manual_checkout_1"))
}

func TestManual_ParseWebhookValid(t *testing.T) {
	m := testManual()
	body := `{"id":"evt_1","type":"invoice.paid","providerRef":"ref-1","invoiceRef":"inv-1","amountCents":2900,"currency":"MYR","paidAt":"2026-09-13T11:00:00Z"}`
	req := webhookRequest(body, m.Sign([]byte(body)))

	evt, err := m.ParseWebhook(req)
	require.NoError(t, err)
	require.Equal(t, "evt_1", evt.ID)
	require.Equal(t, "invoice.paid", evt.Type)
	require.Equal(t, "ref-1", evt.ProviderRef)
	require.Equal(t, "inv-1", evt.InvoiceRef)
	require.Equal(t, 2900, evt.AmountCents)
	require.Equal(t, "MYR", evt.Currency)
	require.Equal(t, time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC), evt.PaidAt)
}

func TestManual_ParseWebhookAcceptsBareHexSignature(t *testing.T) {
	m := testManual()
	body := `{"id":"evt_2","type":"checkout.completed","providerRef":"ref-2"}`
	sig := m.Sign([]byte(body))
	req := webhookRequest(body, sig[len("sha256="):])

	evt, err := m.ParseWebhook(req)
	require.NoError(t, err)
	require.Equal(t, "evt_2", evt.ID)
	require.Equal(t, time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), evt.PaidAt, "paidAt defaults to the clock")
}

func TestManual_ParseWebhookRejectsInvalidSignature(t *testing.T) {
	m := testManual()
	body := `{"id":"evt_3","type":"invoice.paid"}`
	req := webhookRequest(body, m.Sign([]byte(`{"other":"body"}`)))
	_, err := m.ParseWebhook(req)
	require.ErrorIs(t, err, ErrInvalidSignature)
}

func TestManual_ParseWebhookRejectsMissingSignature(t *testing.T) {
	m := testManual()
	body := `{"id":"evt_4","type":"invoice.paid"}`
	_, err := m.ParseWebhook(webhookRequest(body, ""))
	require.ErrorIs(t, err, ErrInvalidSignature)
}

func TestManual_ParseWebhookRejectsMalformedBody(t *testing.T) {
	m := testManual()
	body := `{"id":`
	_, err := m.ParseWebhook(webhookRequest(body, m.Sign([]byte(body))))
	require.ErrorIs(t, err, ErrInvalidPayload)
}

func TestManual_ParseWebhookRejectsMissingFields(t *testing.T) {
	m := testManual()
	body := `{"type":"invoice.paid"}`
	_, err := m.ParseWebhook(webhookRequest(body, m.Sign([]byte(body))))
	require.ErrorIs(t, err, ErrInvalidPayload)
}

func TestManual_EmptySecretRejectsEverything(t *testing.T) {
	m := NewManual("")
	body := `{"id":"evt_5","type":"invoice.paid"}`
	_, err := m.ParseWebhook(webhookRequest(body, "anything"))
	require.ErrorIs(t, err, ErrInvalidSignature)
}
