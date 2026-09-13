package billing

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

const (
	WebhookCheckoutCompleted = "checkout.completed"
	WebhookInvoicePaid       = "invoice.paid"
	WebhookSubscriptionEnded = "subscription.canceled"

	DefaultInterval = "month"

	IntervalMonth = "month"
	IntervalYear  = "year"
)

type Service struct {
	repo     Repository
	provider provider.PaymentProvider
	now      func() time.Time
}

func NewService(repo Repository, p provider.PaymentProvider) *Service {
	return &Service{repo: repo, provider: p, now: time.Now}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func subscriptionNotFound() *apperr.Error {
	return apperr.New("SUBSCRIPTION_NOT_FOUND", "Subscription not found", 404)
}

func planNotFound() *apperr.Error {
	return apperr.New("PLAN_NOT_FOUND", "Plan not found", 404)
}

func alreadyActive() *apperr.Error {
	return apperr.New("SUBSCRIPTION_ALREADY_ACTIVE", "A subscription is already active", 409)
}

func invalidTransition(msg string) *apperr.Error {
	return apperr.New("INVALID_STATUS_TRANSITION", msg, 409)
}

func (s *Service) Plans(ctx context.Context) ([]*Plan, error) {
	plans, err := s.repo.ListPlans(ctx)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return plans, nil
}

func (s *Service) GetSubscription(ctx context.Context, userID uuid.UUID) (*SubscriptionView, error) {
	sub, err := s.repo.GetLatestSubscription(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, apperr.Internal().WithCause(err)
		}
		sub = nil
	}

	if sub != nil && sub.Status.Usable() && sub.CancelAtPeriodEnd &&
		sub.CurrentPeriodEnd != nil && !s.now().UTC().Before(*sub.CurrentPeriodEnd) {
		expired, err := s.repo.SetStatus(ctx, sub.ID, StatusExpired)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		sub = expired
	}

	planID := FreePlanID
	if sub != nil && sub.Status.Usable() {
		planID = sub.PlanID
	}
	plan, err := s.repo.GetPlan(ctx, planID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	active, err := s.repo.CountActiveEvents(ctx, userID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	storage, err := s.repo.UserStorageBytes(ctx, userID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	return &SubscriptionView{
		Subscription: sub,
		Plan:         plan,
		Usage:        Usage{ActiveEvents: active, StorageBytes: storage},
	}, nil
}

func (s *Service) Subscribe(ctx context.Context, userID uuid.UUID, planID, interval string) (*Subscription, *provider.CheckoutSession, error) {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return nil, nil, validationError("planId is required")
	}
	plan, err := s.repo.GetPlan(ctx, planID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, planNotFound()
		}
		return nil, nil, apperr.Internal().WithCause(err)
	}
	if !plan.Active {
		return nil, nil, validationError("Plan is not available")
	}

	interval = strings.TrimSpace(interval)
	if interval == "" {
		interval = plan.Interval
	}
	if interval != IntervalMonth && interval != IntervalYear {
		return nil, nil, validationError("interval must be month or year")
	}

	if _, err := s.repo.GetActiveSubscription(ctx, userID); err == nil {
		return nil, nil, alreadyActive()
	} else if !errors.Is(err, ErrNotFound) {
		return nil, nil, apperr.Internal().WithCause(err)
	}

	now := s.now().UTC().Truncate(time.Second)
	subID := uuid.New()
	amount := planAmount(plan, interval)
	session, err := s.provider.CreateCheckout(ctx, provider.Subscription{
		ID:          subID.String(),
		UserID:      userID.String(),
		PlanID:      plan.ID,
		Status:      string(StatusActive),
		AmountCents: amount,
		Currency:    plan.Currency,
		PeriodEnd:   addInterval(now, interval),
	})
	if err != nil {
		return nil, nil, apperr.Internal().WithCause(err)
	}

	sub, err := s.repo.CreateSubscription(ctx, CreateSubscriptionInput{
		ID:                 subID,
		UserID:             userID,
		PlanID:             plan.ID,
		Status:             StatusActive,
		Provider:           s.provider.Name(),
		ProviderRef:        session.ProviderRef,
		Interval:           interval,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   addInterval(now, interval),
	})
	if err != nil {
		if errors.Is(err, ErrActiveSubscriptionExists) {
			return nil, nil, alreadyActive()
		}
		return nil, nil, apperr.Internal().WithCause(err)
	}

	invoiceID := uuid.New()
	if _, err := s.repo.CreateInvoice(ctx, CreateInvoiceInput{
		ID:             invoiceID,
		UserID:         userID,
		SubscriptionID: &sub.ID,
		AmountCents:    amount,
		Currency:       plan.Currency,
		Status:         InvoiceOpen,
		ProviderRef:    "inv_" + invoiceID.String(),
	}); err != nil {
		return nil, nil, apperr.Internal().WithCause(err)
	}

	return sub, &session, nil
}

func (s *Service) Upgrade(ctx context.Context, userID uuid.UUID, planID string) (*Subscription, error) {
	return s.changePlan(ctx, userID, planID, true)
}

func (s *Service) Downgrade(ctx context.Context, userID uuid.UUID, planID string) (*Subscription, error) {
	return s.changePlan(ctx, userID, planID, false)
}

func (s *Service) changePlan(ctx context.Context, userID uuid.UUID, planID string, up bool) (*Subscription, error) {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return nil, validationError("planId is required")
	}
	target, err := s.repo.GetPlan(ctx, planID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, planNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if !target.Active {
		return nil, validationError("Plan is not available")
	}

	current, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, subscriptionNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if !current.Status.Usable() {
		return nil, invalidTransition("Subscription is not active")
	}
	if current.PlanID == target.ID {
		return nil, validationError("Already on plan " + target.ID)
	}
	currentPlan, err := s.repo.GetPlan(ctx, current.PlanID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if up && target.PriceCents <= currentPlan.PriceCents {
		return nil, validationError("Target plan is not an upgrade")
	}
	if !up && target.PriceCents >= currentPlan.PriceCents {
		return nil, validationError("Target plan is not a downgrade")
	}

	updated, err := s.repo.UpdateSubscriptionPlan(ctx, current.ID, target.ID, current.Interval)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, subscriptionNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Cancel(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	current, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, subscriptionNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if !current.Status.Usable() {
		return nil, invalidTransition("Subscription is not active")
	}
	if current.CancelAtPeriodEnd {
		return current, nil
	}
	if current.ProviderRef != nil && *current.ProviderRef != "" {
		if err := s.provider.Cancel(ctx, *current.ProviderRef); err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
	}
	updated, err := s.repo.SetCancelAtPeriodEnd(ctx, current.ID, true)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Resume(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	current, err := s.repo.GetActiveSubscription(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, subscriptionNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if !current.CancelAtPeriodEnd {
		return nil, invalidTransition("Subscription is not scheduled for cancellation")
	}
	updated, err := s.repo.SetCancelAtPeriodEnd(ctx, current.ID, false)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

type InvoiceList struct {
	Invoices   []*Invoice
	NextCursor string
}

func (s *Service) Invoices(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*InvoiceList, error) {
	c, err := DecodeInvoiceCursor(cursor)
	if err != nil {
		return nil, validationError("Invalid cursor")
	}
	limit = NormalizeLimit(limit)

	items, err := s.repo.ListInvoices(ctx, userID, c, limit)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	result := &InvoiceList{Invoices: items}
	if len(items) == limit && len(items) > 0 {
		last := items[len(items)-1]
		result.NextCursor = EncodeInvoiceCursor(InvoiceCursor{IssuedAt: last.IssuedAt, ID: last.ID})
	}
	return result, nil
}

func (s *Service) HandleWebhook(ctx context.Context, evt provider.WebhookEvent) error {
	if strings.TrimSpace(evt.ID) == "" || strings.TrimSpace(evt.Type) == "" {
		return apperr.New("INVALID_WEBHOOK_PAYLOAD", "Webhook is missing id or type", 400)
	}
	inserted, err := s.repo.RecordWebhookEvent(ctx, s.provider.Name(), evt.ID, evt.Type)
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	if !inserted {
		return nil
	}
	if err := s.applyWebhook(ctx, evt); err != nil {
		// Let the provider retry a transiently failed event.
		_ = s.repo.DeleteWebhookEvent(ctx, s.provider.Name(), evt.ID)
		return err
	}
	return nil
}

func (s *Service) applyWebhook(ctx context.Context, evt provider.WebhookEvent) error {
	switch evt.Type {
	case WebhookCheckoutCompleted, WebhookInvoicePaid:
		var sub *Subscription
		if evt.ProviderRef != "" {
			var err error
			sub, err = s.repo.GetSubscriptionByProviderRef(ctx, evt.ProviderRef)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return apperr.Internal().WithCause(err)
			}
		}
		if sub != nil && sub.Status.Usable() && sub.Status != StatusActive {
			updated, err := s.repo.SetStatus(ctx, sub.ID, StatusActive)
			if err != nil {
				return apperr.Internal().WithCause(err)
			}
			sub = updated
		}
		if evt.InvoiceRef != "" {
			invoice, err := s.repo.GetInvoiceByProviderRef(ctx, evt.InvoiceRef)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return nil
				}
				return apperr.Internal().WithCause(err)
			}
			if invoice.Status != InvoicePaid {
				if err := s.repo.MarkInvoicePaid(ctx, invoice.ID, evt.PaidAt); err != nil {
					return apperr.Internal().WithCause(err)
				}
			}
			return nil
		}
		if sub != nil {
			invoice, err := s.repo.GetLatestOpenInvoice(ctx, sub.ID)
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					return nil
				}
				return apperr.Internal().WithCause(err)
			}
			if err := s.repo.MarkInvoicePaid(ctx, invoice.ID, evt.PaidAt); err != nil {
				return apperr.Internal().WithCause(err)
			}
		}
		return nil
	case WebhookSubscriptionEnded:
		if evt.ProviderRef == "" {
			return nil
		}
		sub, err := s.repo.GetSubscriptionByProviderRef(ctx, evt.ProviderRef)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return apperr.Internal().WithCause(err)
		}
		if _, err := s.repo.SetStatus(ctx, sub.ID, StatusCanceled); err != nil {
			return apperr.Internal().WithCause(err)
		}
		return nil
	default:
		return nil
	}
}

func planAmount(plan *Plan, interval string) int {
	if interval == IntervalYear {
		return plan.PriceCents * 12
	}
	return plan.PriceCents
}

func addInterval(t time.Time, interval string) time.Time {
	if interval == IntervalYear {
		return t.AddDate(1, 0, 0)
	}
	return t.AddDate(0, 1, 0)
}
