package provider

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var (
	ErrInvalidSignature = errors.New("invalid webhook signature")
	ErrInvalidPayload   = errors.New("invalid webhook payload")
)

type Subscription struct {
	ID          string
	UserID      string
	PlanID      string
	Status      string
	ProviderRef string
	AmountCents int
	Currency    string
	PeriodEnd   time.Time
}

type CheckoutSession struct {
	ProviderRef string
	URL         string
	ExpiresAt   time.Time
}

type WebhookEvent struct {
	ID          string
	Type        string
	ProviderRef string
	InvoiceRef  string
	AmountCents int
	Currency    string
	PaidAt      time.Time
}

type PaymentProvider interface {
	Name() string
	CreateCheckout(ctx context.Context, sub Subscription) (CheckoutSession, error)
	Cancel(ctx context.Context, providerRef string) error
	ParseWebhook(r *http.Request) (WebhookEvent, error)
}
