package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEntitlements_FreePlanWhenNoSubscription(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	ent := NewEntitlements(repo)
	ctx := context.Background()
	userID := uuid.New().String()

	maxEvents, err := ent.MaxActiveEvents(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 1, maxEvents)

	maxPhotos, err := ent.MaxPhotosPerEvent(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 500, maxPhotos)

	maxBytes, err := ent.MaxStorageBytes(ctx, userID)
	require.NoError(t, err)
	require.EqualValues(t, 5<<30, maxBytes)
}

func TestEntitlements_ActivePlanOverridesFree(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	userID := uuid.New()
	ref := "prov-1"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: userID, PlanID: "starter", Status: StatusActive,
		Provider: "manual", ProviderRef: &ref, Interval: "month",
	})
	ent := NewEntitlements(repo)

	maxEvents, err := ent.MaxActiveEvents(context.Background(), userID.String())
	require.NoError(t, err)
	require.Equal(t, 5, maxEvents)
}

func TestEntitlements_UnlimitedPlanReturnsZero(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	userID := uuid.New()
	ref := "prov-2"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: userID, PlanID: "business", Status: StatusActive,
		Provider: "manual", ProviderRef: &ref, Interval: "month",
	})
	ent := NewEntitlements(repo)
	ctx := context.Background()

	maxEvents, err := ent.MaxActiveEvents(ctx, userID.String())
	require.NoError(t, err)
	require.Zero(t, maxEvents, "zero means unlimited")

	maxPhotos, err := ent.MaxPhotosPerEvent(ctx, userID.String())
	require.NoError(t, err)
	require.Zero(t, maxPhotos)

	maxBytes, err := ent.MaxStorageBytes(ctx, userID.String())
	require.NoError(t, err)
	require.EqualValues(t, 1<<40, maxBytes)
}

func TestEntitlements_ExpiredSubscriptionFallsBackToFree(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	userID := uuid.New()
	ref := "prov-3"
	repo.subs = append(repo.subs, &Subscription{
		ID: uuid.New(), UserID: userID, PlanID: "business", Status: StatusExpired,
		Provider: "manual", ProviderRef: &ref, Interval: "month",
	})
	ent := NewEntitlements(repo)

	maxEvents, err := ent.MaxActiveEvents(context.Background(), userID.String())
	require.NoError(t, err)
	require.Equal(t, 1, maxEvents)
}

func TestEntitlements_InvalidUserID(t *testing.T) {
	repo := newFakeRepo()
	seedPlans(repo)
	ent := NewEntitlements(repo)
	_, err := ent.MaxActiveEvents(context.Background(), "not-a-uuid")
	require.Error(t, err)
}
