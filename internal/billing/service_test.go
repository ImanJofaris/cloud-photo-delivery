package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected apperr, got %v", err)
	}
	return ae.Code
}

func TestSubscribe_CreatesActiveSubscriptionInvoiceAndCheckout(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()

	sub, checkout, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)
	require.Equal(t, StatusActive, sub.Status)
	require.Equal(t, "starter", sub.PlanID)
	require.Equal(t, "month", sub.Interval)
	require.Equal(t, "manual", sub.Provider)
	require.NotNil(t, sub.ProviderRef)
	require.Contains(t, *sub.ProviderRef, "checkout_")
	require.Equal(t, time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), *sub.CurrentPeriodEnd)
	require.NotNil(t, checkout)
	require.Contains(t, checkout.URL, "manual://checkout/")

	require.Len(t, repo.invoices, 1)
	require.Equal(t, 2900, repo.invoices[0].AmountCents)
	require.Equal(t, InvoiceOpen, repo.invoices[0].Status)
	require.Contains(t, *repo.invoices[0].ProviderRef, "inv_")
}

func TestSubscribe_YearlyIntervalChargesTwelveMonths(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sub, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "year")
	require.NoError(t, err)
	require.Equal(t, "year", sub.Interval)
	require.Equal(t, time.Date(2027, 9, 13, 12, 0, 0, 0, time.UTC), *sub.CurrentPeriodEnd)
	require.Equal(t, 2900*12, repo.invoices[0].AmountCents)
}

func TestSubscribe_RejectsUnknownPlan(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, _, err := svc.Subscribe(context.Background(), uuid.New(), "nonexistent", "")
	require.Equal(t, "PLAN_NOT_FOUND", appErrCode(t, err))
}

func TestSubscribe_RejectsInvalidInterval(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "weekly")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestSubscribe_RejectsWhenAlreadyActive(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)
	_, _, err = svc.Subscribe(context.Background(), userID, "pro", "")
	require.Equal(t, "SUBSCRIPTION_ALREADY_ACTIVE", appErrCode(t, err))
}

func TestUpgrade_ChangesPlanImmediately(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	updated, err := svc.Upgrade(context.Background(), userID, "pro")
	require.NoError(t, err)
	require.Equal(t, "pro", updated.PlanID)
	require.Equal(t, StatusActive, updated.Status)
}

func TestUpgrade_RejectsLowerPricedPlan(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "pro", "")
	require.NoError(t, err)

	_, err = svc.Upgrade(context.Background(), userID, "starter")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestUpgrade_RequiresSubscription(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Upgrade(context.Background(), uuid.New(), "pro")
	require.Equal(t, "SUBSCRIPTION_NOT_FOUND", appErrCode(t, err))
}

func TestDowngrade_SoftlyChangesPlanEvenOverLimits(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "pro", "")
	require.NoError(t, err)

	repo.activeEvents = 50
	repo.storageBytes = 1 << 40

	updated, err := svc.Downgrade(context.Background(), userID, "starter")
	require.NoError(t, err, "downgrade must never be blocked by existing data")
	require.Equal(t, "starter", updated.PlanID)
}

func TestDowngrade_RejectsHigherPricedPlan(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	_, err = svc.Downgrade(context.Background(), userID, "pro")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestCancel_SetsPeriodEndAndNotifiesProvider(t *testing.T) {
	svc, _, prov := newTestService(t)
	userID := uuid.New()
	sub, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	canceled, err := svc.Cancel(context.Background(), userID)
	require.NoError(t, err)
	require.True(t, canceled.CancelAtPeriodEnd)
	require.Equal(t, StatusActive, canceled.Status, "access continues until period end")
	require.Equal(t, []string{*sub.ProviderRef}, prov.cancelCalls)
}

func TestCancel_IsIdempotent(t *testing.T) {
	svc, _, prov := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	_, err = svc.Cancel(context.Background(), userID)
	require.NoError(t, err)
	_, err = svc.Cancel(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, prov.cancelCalls, 1)
}

func TestResume_ClearsCancellation(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)
	_, err = svc.Cancel(context.Background(), userID)
	require.NoError(t, err)

	resumed, err := svc.Resume(context.Background(), userID)
	require.NoError(t, err)
	require.False(t, resumed.CancelAtPeriodEnd)
}

func TestResume_RejectsWhenNotScheduledForCancellation(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)
	_, err = svc.Resume(context.Background(), userID)
	require.Equal(t, "INVALID_STATUS_TRANSITION", appErrCode(t, err))
}

func TestGetSubscription_LazilyExpiresAfterCanceledPeriodEnd(t *testing.T) {
	svc, _, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)
	_, err = svc.Cancel(context.Background(), userID)
	require.NoError(t, err)

	svc.SetClock(func() time.Time { return time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC) })
	view, err := svc.GetSubscription(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, StatusExpired, view.Subscription.Status)
	require.Equal(t, FreePlanID, view.Plan.ID, "expired subscriptions fall back to the free plan")
}

func TestGetSubscription_NoSubscriptionShowsFreePlanAndUsage(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.activeEvents = 2
	repo.storageBytes = 1234

	view, err := svc.GetSubscription(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Nil(t, view.Subscription)
	require.Equal(t, FreePlanID, view.Plan.ID)
	require.Equal(t, 2, view.Usage.ActiveEvents)
	require.EqualValues(t, 1234, view.Usage.StorageBytes)
}

func TestInvoices_CursorPagination(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	for i := 0; i < 3; i++ {
		_, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
			ID: uuid.New(), UserID: userID, AmountCents: 100, Currency: "MYR",
			Status: InvoiceOpen, ProviderRef: "inv_" + uuid.NewString(),
		})
		require.NoError(t, err)
	}

	first, err := svc.Invoices(context.Background(), userID, "", 2)
	require.NoError(t, err)
	require.Len(t, first.Invoices, 2)
	require.NotEmpty(t, first.NextCursor)

	second, err := svc.Invoices(context.Background(), userID, first.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, second.Invoices, 1)
	require.Empty(t, second.NextCursor)
	require.Greater(t, first.Invoices[0].IssuedAt, second.Invoices[0].IssuedAt)

	_, err = svc.Invoices(context.Background(), userID, "not-a-cursor", 2)
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestInvoices_TenantIsolation(t *testing.T) {
	svc, repo, _ := newTestService(t)
	owner := uuid.New()
	_, err := repo.CreateInvoice(context.Background(), CreateInvoiceInput{
		ID: uuid.New(), UserID: owner, AmountCents: 100, Currency: "MYR", Status: InvoiceOpen,
	})
	require.NoError(t, err)

	result, err := svc.Invoices(context.Background(), uuid.New(), "", 20)
	require.NoError(t, err)
	require.Empty(t, result.Invoices)
}

func TestHandleWebhook_InvoicePaidAppliesOnce(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	sub, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	evt := provider.WebhookEvent{
		ID:          "evt_1",
		Type:        WebhookInvoicePaid,
		ProviderRef: *sub.ProviderRef,
		InvoiceRef:  *repo.invoices[0].ProviderRef,
		PaidAt:      time.Date(2026, 9, 13, 12, 5, 0, 0, time.UTC),
	}
	require.NoError(t, svc.HandleWebhook(context.Background(), evt))
	require.Equal(t, 1, repo.markPaidCalls)
	require.Equal(t, InvoicePaid, repo.invoices[0].Status)

	require.NoError(t, svc.HandleWebhook(context.Background(), evt))
	require.Equal(t, 1, repo.markPaidCalls, "duplicate event id must be ignored")
	require.Len(t, repo.webhooks, 1)
}

func TestHandleWebhook_InvoicePaidWithoutRefMarksLatestOpenInvoice(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sub, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "")
	require.NoError(t, err)

	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt_2", Type: WebhookInvoicePaid, ProviderRef: *sub.ProviderRef,
		PaidAt: time.Date(2026, 9, 13, 12, 5, 0, 0, time.UTC),
	}))
	require.Equal(t, InvoicePaid, repo.invoices[0].Status)
}

func TestHandleWebhook_CheckoutCompletedActivatesPastDue(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	ref := "prov-pastdue"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: userID, PlanID: "starter", Status: StatusPastDue,
		Provider: "manual", ProviderRef: &ref, Interval: "month",
	})

	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt_3", Type: WebhookCheckoutCompleted, ProviderRef: ref,
	}))
	require.Equal(t, StatusActive, repo.subs[0].Status)
}

func TestHandleWebhook_SubscriptionCanceledEndsImmediately(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	sub, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt_4", Type: WebhookSubscriptionEnded, ProviderRef: *sub.ProviderRef,
	}))
	require.Equal(t, StatusCanceled, repo.subs[0].Status)
}

func TestHandleWebhook_UnknownEventIsAcknowledged(t *testing.T) {
	svc, repo, _ := newTestService(t)
	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt_5", Type: "customer.updated",
	}))
	require.Len(t, repo.webhooks, 1)
}

func TestHandleWebhook_RejectsMissingFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	err := svc.HandleWebhook(context.Background(), provider.WebhookEvent{ID: ""})
	require.Equal(t, "INVALID_WEBHOOK_PAYLOAD", appErrCode(t, err))
}

func TestPlans_RepoErrorIsInternal(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.listPlansErr = errors.New("db down")
	_, err := svc.Plans(context.Background())
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestChangePlan_ErrorBranches(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	_, err = svc.Upgrade(context.Background(), userID, "missing")
	require.Equal(t, "PLAN_NOT_FOUND", appErrCode(t, err))

	repo.plans["legacy"] = &Plan{ID: "legacy", Name: "Legacy", PriceCents: 9900, Currency: "MYR", Interval: IntervalMonth, Active: false}
	_, err = svc.Upgrade(context.Background(), userID, "legacy")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))

	_, err = svc.Upgrade(context.Background(), userID, "starter")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))

	_, err = svc.Upgrade(context.Background(), userID, "  ")
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestChangePlan_NoSubscription(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Downgrade(context.Background(), uuid.New(), FreePlanID)
	require.Equal(t, "SUBSCRIPTION_NOT_FOUND", appErrCode(t, err))
}

func TestSubscribe_ProviderErrorIsInternal(t *testing.T) {
	svc, _, prov := newTestService(t)
	prov.createErr = errors.New("gateway down")
	_, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "")
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestSubscribe_InvoiceErrorIsInternal(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.createInvErr = errors.New("db down")
	_, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "")
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestSubscribe_ConcurrentActiveSubscriptionIsConflict(t *testing.T) {
	svc, repo, _ := newTestService(t)
	userID := uuid.New()
	ref := "existing"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: userID, PlanID: "starter", Status: StatusActive,
		Provider: "manual", ProviderRef: &ref, Interval: IntervalMonth,
	})
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.Equal(t, "SUBSCRIPTION_ALREADY_ACTIVE", appErrCode(t, err))
}

func TestCancel_ProviderErrorIsInternal(t *testing.T) {
	svc, _, prov := newTestService(t)
	userID := uuid.New()
	_, _, err := svc.Subscribe(context.Background(), userID, "starter", "")
	require.NoError(t, err)

	prov.cancelErr = errors.New("gateway down")
	_, err = svc.Cancel(context.Background(), userID)
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestHandleWebhook_RecordErrorIsInternal(t *testing.T) {
	svc, repo, _ := newTestService(t)
	repo.recordWebhookErr = errors.New("db down")
	err := svc.HandleWebhook(context.Background(), provider.WebhookEvent{ID: "evt-x", Type: WebhookInvoicePaid})
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestHandleWebhook_CompensatesWhenApplyFails(t *testing.T) {
	svc, repo, _ := newTestService(t)
	ref := "prov-fail"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: uuid.New(), PlanID: "starter", Status: StatusPastDue,
		Provider: "manual", ProviderRef: &ref, Interval: IntervalMonth,
	})
	repo.setStatusErr = errors.New("db down")

	err := svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt-fail", Type: WebhookCheckoutCompleted, ProviderRef: ref,
	})
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	require.Empty(t, repo.webhooks, "failed events are removed so the provider can retry")
}

func TestApplyWebhook_InvoiceRefNotFoundIsIgnored(t *testing.T) {
	svc, repo, _ := newTestService(t)
	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt-missing", Type: WebhookInvoicePaid, InvoiceRef: "missing",
	}))
	require.Zero(t, repo.markPaidCalls)
}

func TestApplyWebhook_MarkPaidErrorIsInternal(t *testing.T) {
	svc, repo, _ := newTestService(t)
	sub, _, err := svc.Subscribe(context.Background(), uuid.New(), "starter", "")
	require.NoError(t, err)
	repo.markPaidErr = errors.New("db down")

	err = svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt-mark", Type: WebhookInvoicePaid, ProviderRef: *sub.ProviderRef,
	})
	require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
}

func TestApplyWebhook_SubscriptionCanceledUnknownRefIsIgnored(t *testing.T) {
	svc, _, _ := newTestService(t)
	require.NoError(t, svc.HandleWebhook(context.Background(), provider.WebhookEvent{
		ID: "evt-cancel", Type: WebhookSubscriptionEnded, ProviderRef: "missing",
	}))
}
