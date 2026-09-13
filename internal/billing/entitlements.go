package billing

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Entitlements implements limits.PlanLimits for the events and uploads
// domains. It resolves the tenant's effective plan limits from the database
// so those domains never import billing.
type Entitlements struct {
	repo Repository
}

func NewEntitlements(repo Repository) *Entitlements {
	return &Entitlements{repo: repo}
}

func (e *Entitlements) MaxActiveEvents(ctx context.Context, userID string) (int, error) {
	l, err := e.limitsFor(ctx, userID)
	if err != nil {
		return 0, err
	}
	return l.ActiveEvents, nil
}

func (e *Entitlements) MaxPhotosPerEvent(ctx context.Context, userID string) (int, error) {
	l, err := e.limitsFor(ctx, userID)
	if err != nil {
		return 0, err
	}
	return l.PhotosPerEvent, nil
}

func (e *Entitlements) MaxStorageBytes(ctx context.Context, userID string) (int64, error) {
	l, err := e.limitsFor(ctx, userID)
	if err != nil {
		return 0, err
	}
	return l.StorageBytes, nil
}

func (e *Entitlements) limitsFor(ctx context.Context, userID string) (PlanLimits, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return PlanLimits{}, err
	}
	sub, err := e.repo.GetActiveSubscription(ctx, id)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return PlanLimits{}, err
		}
		plan, err := e.repo.GetPlan(ctx, FreePlanID)
		if err != nil {
			return PlanLimits{}, err
		}
		return plan.Limits, nil
	}
	plan, err := e.repo.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return PlanLimits{}, err
	}
	return plan.Limits, nil
}
