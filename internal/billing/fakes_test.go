package billing

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
)

type fakeRepo struct {
	plans            map[string]*Plan
	subs             []*Subscription
	invoices         []*Invoice
	webhooks         map[string]string
	activeEvents     int
	storageBytes     int64
	markPaidCalls    int
	createSubErr     error
	listPlansErr     error
	createInvErr     error
	setStatusErr     error
	markPaidErr      error
	recordWebhookErr error
	invoiceSeq       int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		plans:    map[string]*Plan{},
		webhooks: map[string]string{},
	}
}

func copySubscription(s *Subscription) *Subscription {
	c := *s
	if s.ProviderRef != nil {
		ref := *s.ProviderRef
		c.ProviderRef = &ref
	}
	if s.CurrentPeriodStart != nil {
		t := *s.CurrentPeriodStart
		c.CurrentPeriodStart = &t
	}
	if s.CurrentPeriodEnd != nil {
		t := *s.CurrentPeriodEnd
		c.CurrentPeriodEnd = &t
	}
	return &c
}

func copyInvoice(inv *Invoice) *Invoice {
	c := *inv
	if inv.ProviderRef != nil {
		ref := *inv.ProviderRef
		c.ProviderRef = &ref
	}
	if inv.SubscriptionID != nil {
		id := *inv.SubscriptionID
		c.SubscriptionID = &id
	}
	if inv.PaidAt != nil {
		t := *inv.PaidAt
		c.PaidAt = &t
	}
	return &c
}

func (f *fakeRepo) ListPlans(ctx context.Context) ([]*Plan, error) {
	if f.listPlansErr != nil {
		return nil, f.listPlansErr
	}
	out := make([]*Plan, 0, len(f.plans))
	for _, p := range f.plans {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PriceCents < out[j].PriceCents })
	return out, nil
}

func (f *fakeRepo) GetPlan(ctx context.Context, id string) (*Plan, error) {
	p, ok := f.plans[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) findActive(userID uuid.UUID) *Subscription {
	for i := len(f.subs) - 1; i >= 0; i-- {
		s := f.subs[i]
		if s.UserID == userID && s.Status.Usable() {
			return s
		}
	}
	return nil
}

func (f *fakeRepo) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	s := f.findActive(userID)
	if s == nil {
		return nil, ErrNotFound
	}
	return copySubscription(s), nil
}

func (f *fakeRepo) GetLatestSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	for i := len(f.subs) - 1; i >= 0; i-- {
		if f.subs[i].UserID == userID {
			return copySubscription(f.subs[i]), nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) GetSubscriptionByProviderRef(ctx context.Context, ref string) (*Subscription, error) {
	for _, s := range f.subs {
		if s.ProviderRef != nil && *s.ProviderRef == ref {
			return copySubscription(s), nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error) {
	if f.createSubErr != nil {
		return nil, f.createSubErr
	}
	if f.findActive(in.UserID) != nil {
		return nil, ErrActiveSubscriptionExists
	}
	ref := in.ProviderRef
	s := &Subscription{
		ID:                 in.ID,
		UserID:             in.UserID,
		PlanID:             in.PlanID,
		Status:             in.Status,
		Provider:           in.Provider,
		ProviderRef:        &ref,
		Interval:           in.Interval,
		CurrentPeriodStart: &in.CurrentPeriodStart,
		CurrentPeriodEnd:   &in.CurrentPeriodEnd,
		CreatedAt:          in.CurrentPeriodStart,
		UpdatedAt:          in.CurrentPeriodStart,
	}
	f.subs = append(f.subs, s)
	return copySubscription(s), nil
}

func (f *fakeRepo) findSub(id uuid.UUID) *Subscription {
	for _, s := range f.subs {
		if s.ID == id {
			return s
		}
	}
	return nil
}

func (f *fakeRepo) UpdateSubscriptionPlan(ctx context.Context, id uuid.UUID, planID, interval string) (*Subscription, error) {
	s := f.findSub(id)
	if s == nil {
		return nil, ErrNotFound
	}
	s.PlanID = planID
	s.Interval = interval
	return copySubscription(s), nil
}

func (f *fakeRepo) SetCancelAtPeriodEnd(ctx context.Context, id uuid.UUID, cancel bool) (*Subscription, error) {
	s := f.findSub(id)
	if s == nil {
		return nil, ErrNotFound
	}
	s.CancelAtPeriodEnd = cancel
	return copySubscription(s), nil
}

func (f *fakeRepo) SetStatus(ctx context.Context, id uuid.UUID, status Status) (*Subscription, error) {
	if f.setStatusErr != nil {
		return nil, f.setStatusErr
	}
	s := f.findSub(id)
	if s == nil {
		return nil, ErrNotFound
	}
	s.Status = status
	return copySubscription(s), nil
}

func (f *fakeRepo) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (*Invoice, error) {
	if f.createInvErr != nil {
		return nil, f.createInvErr
	}
	f.invoiceSeq++
	inv := &Invoice{
		ID:             in.ID,
		UserID:         in.UserID,
		SubscriptionID: in.SubscriptionID,
		AmountCents:    in.AmountCents,
		Currency:       in.Currency,
		Status:         in.Status,
		IssuedAt:       time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC).Add(time.Duration(f.invoiceSeq) * time.Second),
	}
	if in.ProviderRef != "" {
		ref := in.ProviderRef
		inv.ProviderRef = &ref
	}
	f.invoices = append(f.invoices, inv)
	return copyInvoice(inv), nil
}

func (f *fakeRepo) GetInvoiceByProviderRef(ctx context.Context, ref string) (*Invoice, error) {
	for _, inv := range f.invoices {
		if inv.ProviderRef != nil && *inv.ProviderRef == ref {
			return copyInvoice(inv), nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) GetLatestOpenInvoice(ctx context.Context, subscriptionID uuid.UUID) (*Invoice, error) {
	for i := len(f.invoices) - 1; i >= 0; i-- {
		inv := f.invoices[i]
		if inv.SubscriptionID != nil && *inv.SubscriptionID == subscriptionID && inv.Status == InvoiceOpen {
			return copyInvoice(inv), nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) MarkInvoicePaid(ctx context.Context, id uuid.UUID, paidAt time.Time) error {
	if f.markPaidErr != nil {
		return f.markPaidErr
	}
	f.markPaidCalls++
	for _, inv := range f.invoices {
		if inv.ID == id {
			inv.Status = InvoicePaid
			t := paidAt
			inv.PaidAt = &t
			return nil
		}
	}
	return ErrNotFound
}

func (f *fakeRepo) ListInvoices(ctx context.Context, userID uuid.UUID, cursor *InvoiceCursor, limit int) ([]*Invoice, error) {
	out := make([]*Invoice, 0)
	for _, inv := range f.invoices {
		if inv.UserID != userID {
			continue
		}
		if cursor != nil {
			if inv.IssuedAt.After(cursor.IssuedAt) || (inv.IssuedAt.Equal(cursor.IssuedAt) && inv.ID.String() >= cursor.ID.String()) {
				continue
			}
		}
		out = append(out, copyInvoice(inv))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IssuedAt.Equal(out[j].IssuedAt) {
			return out[i].ID.String() > out[j].ID.String()
		}
		return out[i].IssuedAt.After(out[j].IssuedAt)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeRepo) RecordWebhookEvent(ctx context.Context, providerName, providerRef, eventType string) (bool, error) {
	if f.recordWebhookErr != nil {
		return false, f.recordWebhookErr
	}
	key := providerName + ":" + providerRef
	if _, ok := f.webhooks[key]; ok {
		return false, nil
	}
	f.webhooks[key] = eventType
	return true, nil
}

func (f *fakeRepo) DeleteWebhookEvent(ctx context.Context, providerName, providerRef string) error {
	delete(f.webhooks, providerName+":"+providerRef)
	return nil
}

func (f *fakeRepo) CountLiveEvents(ctx context.Context, userID uuid.UUID) (int, error) {
	return f.activeEvents, nil
}

func (f *fakeRepo) UserStorageBytes(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.storageBytes, nil
}

type fakeProvider struct {
	name        string
	ref         string
	createErr   error
	cancelErr   error
	cancelCalls []string
	webhook     provider.WebhookEvent
	parseErr    error
}

func (p *fakeProvider) Name() string {
	if p.name == "" {
		return "manual"
	}
	return p.name
}

func (p *fakeProvider) CreateCheckout(ctx context.Context, sub provider.Subscription) (provider.CheckoutSession, error) {
	if p.createErr != nil {
		return provider.CheckoutSession{}, p.createErr
	}
	ref := p.ref
	if ref == "" {
		ref = "checkout_" + sub.ID
	}
	return provider.CheckoutSession{
		ProviderRef: ref,
		URL:         "manual://checkout/" + ref,
		ExpiresAt:   time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (p *fakeProvider) Cancel(ctx context.Context, providerRef string) error {
	if p.cancelErr != nil {
		return p.cancelErr
	}
	p.cancelCalls = append(p.cancelCalls, providerRef)
	return nil
}

func (p *fakeProvider) ParseWebhook(r *http.Request) (provider.WebhookEvent, error) {
	return p.webhook, p.parseErr
}

func testPlan(id string, price int, limits PlanLimits) *Plan {
	return &Plan{
		ID:         id,
		Name:       fmt.Sprintf("Plan %s", id),
		PriceCents: price,
		Currency:   "MYR",
		Interval:   IntervalMonth,
		Limits:     limits,
		Active:     true,
	}
}

func seedPlans(repo *fakeRepo) {
	repo.plans[FreePlanID] = testPlan(FreePlanID, 0, PlanLimits{Events: 1, PhotosPerEvent: 500, StorageBytes: 5 << 30, RetentionDays: 7})
	repo.plans["starter"] = testPlan("starter", 2900, PlanLimits{Events: 5, PhotosPerEvent: 5000, StorageBytes: 50 << 30, RetentionDays: 30, Branding: true, OriginalDownloads: true})
	repo.plans["pro"] = testPlan("pro", 5900, PlanLimits{Events: 0, PhotosPerEvent: 20000, StorageBytes: 500 << 30, RetentionDays: 90, APIAccess: true, Branding: true, OriginalDownloads: true})
	repo.plans["business"] = testPlan("business", 9900, PlanLimits{Events: 0, PhotosPerEvent: 0, StorageBytes: 1 << 40, RetentionDays: 365, APIAccess: true, Branding: true, OriginalDownloads: true})
}

func newTestService(t *testing.T) (*Service, *fakeRepo, *fakeProvider) {
	t.Helper()
	repo := newFakeRepo()
	seedPlans(repo)
	prov := &fakeProvider{}
	svc := NewService(repo, prov)
	svc.SetClock(func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) })
	return svc, repo, prov
}
