package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ManualName      = "manual"
	SignatureHeader = "X-Billing-Signature"
)

// Manual is the offline development provider. It makes no network calls:
// checkout refs are generated locally and webhooks are verified with an HMAC
// over the raw body using the configured secret.
type Manual struct {
	secret []byte
	now    func() time.Time
}

func NewManual(secret string) *Manual {
	return &Manual{secret: []byte(secret), now: time.Now}
}

func (m *Manual) SetClock(now func() time.Time) { m.now = now }

func (m *Manual) Name() string { return ManualName }

func (m *Manual) CreateCheckout(ctx context.Context, sub Subscription) (CheckoutSession, error) {
	ref := "manual_checkout_" + uuid.NewString()
	return CheckoutSession{
		ProviderRef: ref,
		URL:         "manual://checkout/" + ref,
		ExpiresAt:   m.now().UTC().Add(24 * time.Hour),
	}, nil
}

func (m *Manual) Cancel(ctx context.Context, providerRef string) error { return nil }

type manualPayload struct {
	ID          string     `json:"id"`
	Type        string     `json:"type"`
	ProviderRef string     `json:"providerRef"`
	InvoiceRef  string     `json:"invoiceRef"`
	AmountCents int        `json:"amountCents"`
	Currency    string     `json:"currency"`
	PaidAt      *time.Time `json:"paidAt"`
}

func (m *Manual) ParseWebhook(r *http.Request) (WebhookEvent, error) {
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return WebhookEvent{}, ErrInvalidPayload
	}
	if !m.validSignature(r.Header.Get(SignatureHeader), body) {
		return WebhookEvent{}, ErrInvalidSignature
	}
	var payload manualPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return WebhookEvent{}, ErrInvalidPayload
	}
	if strings.TrimSpace(payload.ID) == "" || strings.TrimSpace(payload.Type) == "" {
		return WebhookEvent{}, ErrInvalidPayload
	}
	paidAt := m.now().UTC()
	if payload.PaidAt != nil {
		paidAt = payload.PaidAt.UTC()
	}
	return WebhookEvent{
		ID:          payload.ID,
		Type:        payload.Type,
		ProviderRef: payload.ProviderRef,
		InvoiceRef:  payload.InvoiceRef,
		AmountCents: payload.AmountCents,
		Currency:    payload.Currency,
		PaidAt:      paidAt,
	}, nil
}

func (m *Manual) validSignature(header string, body []byte) bool {
	if len(m.secret) == 0 {
		return false
	}
	raw := strings.TrimPrefix(strings.TrimSpace(header), "sha256=")
	got, err := hex.DecodeString(raw)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// Sign returns the header value a caller should send for a body. It exists so
// local clients and tests can produce valid requests; services never log it.
func (m *Manual) Sign(body []byte) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
