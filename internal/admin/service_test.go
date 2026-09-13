package admin

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

func TestService_Stats(t *testing.T) {
	repo := newFakeRepo()
	repo.stats = &Stats{Users: 4, Events: 9, Photos: 80, StorageBytes: 1024, RevenueCents: 9900, Subscriptions: 2}
	svc := NewService(repo)

	stats, err := svc.Stats(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(4), stats.Users)
	require.Equal(t, int64(9), stats.Events)
	require.Equal(t, int64(80), stats.Photos)
	require.Equal(t, int64(1024), stats.StorageBytes)
	require.Equal(t, int64(9900), stats.RevenueCents)
	require.Equal(t, int64(2), stats.Subscriptions)
}

func TestService_Stats_RepoErrorIsInternal(t *testing.T) {
	repo := newFakeRepo()
	repo.statsErr = errors.New("boom")
	_, err := NewService(repo).Stats(context.Background())

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, http.StatusInternalServerError, appErr.HTTPStatus)
	require.Equal(t, "INTERNAL_ERROR", appErr.Code)
	require.NotContains(t, appErr.Message, "boom")
}

func TestService_ListUsers_Paginates(t *testing.T) {
	repo := newFakeRepo()
	base := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	last := &UserSummary{ID: uuid.New(), Email: "b@example.com", CreatedAt: base}
	repo.users = []*UserSummary{
		{ID: uuid.New(), Email: "a@example.com", CreatedAt: base.Add(time.Minute)},
		last,
	}
	svc := NewService(repo)

	result, err := svc.ListUsers(context.Background(), "", 2)
	require.NoError(t, err)
	require.Len(t, result.Users, 2)
	require.Equal(t, 2, repo.lastUserLimit)
	require.Nil(t, repo.lastUserCursor)
	require.NotEmpty(t, result.NextCursor)

	decoded, err := DecodeCursor(result.NextCursor)
	require.NoError(t, err)
	require.Equal(t, last.ID, decoded.ID)
}

func TestService_ListUsers_NoCursorWhenShortPage(t *testing.T) {
	repo := newFakeRepo()
	repo.users = []*UserSummary{{ID: uuid.New(), CreatedAt: time.Now()}}
	result, err := NewService(repo).ListUsers(context.Background(), "", 20)
	require.NoError(t, err)
	require.Empty(t, result.NextCursor)
}

func TestService_ListUsers_InvalidCursor(t *testing.T) {
	_, err := NewService(newFakeRepo()).ListUsers(context.Background(), "%%%", 20)

	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, "VALIDATION_ERROR", appErr.Code)
	require.Equal(t, http.StatusUnprocessableEntity, appErr.HTTPStatus)
}

func TestService_ListUsers_NormalizesLimit(t *testing.T) {
	repo := newFakeRepo()
	_, err := NewService(repo).ListUsers(context.Background(), "", MaxLimit+50)
	require.NoError(t, err)
	require.Equal(t, MaxLimit, repo.lastUserLimit)
}

func TestService_ListSubscriptions_Paginates(t *testing.T) {
	repo := newFakeRepo()
	base := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	first := &SubscriptionSummary{ID: uuid.New(), PlanID: "pro", Status: "active", CreatedAt: base.Add(time.Minute)}
	second := &SubscriptionSummary{ID: uuid.New(), PlanID: "free", Status: "trialing", CreatedAt: base}
	repo.subs = []*SubscriptionSummary{first, second}
	svc := NewService(repo)

	result, err := svc.ListSubscriptions(context.Background(), "", 2)
	require.NoError(t, err)
	require.Len(t, result.Subscriptions, 2)
	require.NotEmpty(t, result.NextCursor)

	decoded, err := DecodeCursor(result.NextCursor)
	require.NoError(t, err)
	require.Equal(t, second.ID, decoded.ID)
}

func TestService_ListSubscriptions_InvalidCursor(t *testing.T) {
	_, err := NewService(newFakeRepo()).ListSubscriptions(context.Background(), "nope", 20)
	require.Error(t, err)
}

func TestService_Health_OK(t *testing.T) {
	repo := newFakeRepo()
	oldest := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	repo.queue = &QueueHealth{Pending: 2, Running: 1, OldestPendingAt: &oldest}
	svc := NewService(repo)
	now := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	svc.SetClock(func() time.Time { return now })

	health, err := svc.Health(context.Background())
	require.NoError(t, err)
	require.Equal(t, StatusOK, health.Status)
	require.Equal(t, int64(3), health.QueueDepth())
	require.Equal(t, now, health.CheckedAt)
}

func TestService_Health_DegradedOnFailedJobs(t *testing.T) {
	repo := newFakeRepo()
	repo.queue = &QueueHealth{Failed: 1}
	health, err := NewService(repo).Health(context.Background())
	require.NoError(t, err)
	require.Equal(t, StatusDegraded, health.Status)
}

func TestService_Health_RepoError(t *testing.T) {
	repo := newFakeRepo()
	repo.queueErr = errors.New("db down")
	_, err := NewService(repo).Health(context.Background())
	require.Error(t, err)
}

func TestService_IsAdmin(t *testing.T) {
	repo := newFakeRepo()
	adminID, otherID := uuid.New(), uuid.New()
	repo.admins[adminID] = true
	svc := NewService(repo)

	ok, err := svc.IsAdmin(context.Background(), adminID)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = svc.IsAdmin(context.Background(), otherID)
	require.NoError(t, err)
	require.False(t, ok)

	repo.isAdminErr = errors.New("boom")
	_, err = svc.IsAdmin(context.Background(), adminID)
	require.Error(t, err)
}
