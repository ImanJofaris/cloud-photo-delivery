package devices

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*Service, *fakeRepo, uuid.UUID) {
	t.Helper()
	repo := newFakeRepo()
	svc := NewService(repo)
	svc.SetClock(func() time.Time { return time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC) })
	return svc, repo, uuid.New()
}

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	return appErr.Code
}

func TestCreate_StoresHashNotRawKey(t *testing.T) {
	svc, repo, userID := newTestService(t)

	device, raw, err := svc.Create(context.Background(), userID, CreateParams{Name: "  Booth 1  "})
	require.NoError(t, err)
	require.Equal(t, "Booth 1", device.Name)
	require.NotEqual(t, raw, device.KeyHash)
	require.Equal(t, HashKey(raw), device.KeyHash)
	require.NotContains(t, device.KeyHash, raw)

	stored := repo.byID[device.ID]
	require.NotNil(t, stored)
	require.NotEqual(t, raw, stored.KeyHash)
	require.Equal(t, device.KeyPrefix, stored.KeyPrefix)
}

func TestCreate_AssignedEventMustBeOwned(t *testing.T) {
	svc, repo, userID := newTestService(t)
	eventID := uuid.New()
	repo.events[eventID] = userID

	device, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Booth", AssignedEventID: &eventID})
	require.NoError(t, err)
	require.Equal(t, &eventID, device.AssignedEventID)

	foreign := uuid.New()
	_, _, err = svc.Create(context.Background(), userID, CreateParams{Name: "Booth", AssignedEventID: &foreign})
	require.Error(t, err)
	require.Equal(t, "EVENT_NOT_FOUND", appErrCode(t, err))
}

func TestCreate_Validation(t *testing.T) {
	svc, _, userID := newTestService(t)

	for name, params := range map[string]CreateParams{
		"empty":    {Name: "   "},
		"too long": {Name: string(make([]byte, maxNameLength+1))},
	} {
		_, _, err := svc.Create(context.Background(), userID, params)
		require.Error(t, err, name)
		require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err), name)
	}
}

func TestList_ScopesToUser(t *testing.T) {
	svc, repo, userID := newTestService(t)
	other := uuid.New()
	repo.seed(userID, "Mine", nil)
	repo.seed(other, "Theirs", nil)

	items, err := svc.List(context.Background(), userID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Mine", items[0].Name)
}

func TestRename_TenantScoped(t *testing.T) {
	svc, repo, userID := newTestService(t)
	other := uuid.New()
	device, _ := repo.seed(userID, "Old", nil)

	updated, err := svc.Rename(context.Background(), userID, device.ID, "New")
	require.NoError(t, err)
	require.Equal(t, "New", updated.Name)

	_, err = svc.Rename(context.Background(), other, device.ID, "Hacked")
	require.Error(t, err)
	require.Equal(t, "DEVICE_NOT_FOUND", appErrCode(t, err))

	_, err = svc.Rename(context.Background(), userID, device.ID, " ")
	require.Error(t, err)
	require.Equal(t, "VALIDATION_ERROR", appErrCode(t, err))
}

func TestRotate_OldKeyStopsWorking(t *testing.T) {
	svc, repo, userID := newTestService(t)
	device, oldRaw := repo.seed(userID, "Booth", nil)

	updated, newRaw, err := svc.Rotate(context.Background(), userID, device.ID)
	require.NoError(t, err)
	require.NotEqual(t, oldRaw, newRaw)
	require.Equal(t, HashKey(newRaw), updated.KeyHash)

	_, err = svc.Authenticate(context.Background(), oldRaw)
	require.Error(t, err)
	require.Equal(t, "INVALID_DEVICE_KEY", appErrCode(t, err))

	authed, err := svc.Authenticate(context.Background(), newRaw)
	require.NoError(t, err)
	require.Equal(t, device.ID, authed.ID)
}

func TestRotate_RejectsRevoked(t *testing.T) {
	svc, repo, userID := newTestService(t)
	device, _ := repo.seed(userID, "Booth", nil)
	require.NoError(t, svc.Revoke(context.Background(), userID, device.ID))

	_, _, err := svc.Rotate(context.Background(), userID, device.ID)
	require.Error(t, err)
	require.Equal(t, "DEVICE_REVOKED", appErrCode(t, err))
}

func TestRevoke_TenantScoped(t *testing.T) {
	svc, repo, userID := newTestService(t)
	other := uuid.New()
	device, goodRaw := repo.seed(userID, "Booth", nil)

	err := svc.Revoke(context.Background(), other, device.ID)
	require.Error(t, err)
	require.Equal(t, "DEVICE_NOT_FOUND", appErrCode(t, err))
	_, err = svc.Authenticate(context.Background(), goodRaw)
	require.NoError(t, err)

	require.NoError(t, svc.Revoke(context.Background(), userID, device.ID))
	_, err = svc.Authenticate(context.Background(), goodRaw)
	require.Error(t, err)
	require.Equal(t, "DEVICE_REVOKED", appErrCode(t, err))
}

func TestAuthenticate_InvalidKeys(t *testing.T) {
	svc, repo, userID := newTestService(t)
	device, raw := repo.seed(userID, "Booth", nil)
	require.Equal(t, device.ID, mustAuth(t, svc, raw).ID)

	for name, key := range map[string]string{
		"malformed":      "not-a-key",
		"wrong secret":   "cpd_live_" + device.KeyPrefix + "_wrongsecret",
		"unknown prefix": "cpd_live_ffffffffffff_secret",
	} {
		_, err := svc.Authenticate(context.Background(), key)
		require.Error(t, err, name)
		require.Equal(t, "INVALID_DEVICE_KEY", appErrCode(t, err), name)
	}
}

func TestAuthenticate_TouchesLastUsedOnSuccess(t *testing.T) {
	svc, repo, userID := newTestService(t)
	device, raw := repo.seed(userID, "Booth", nil)

	_, err := svc.Authenticate(context.Background(), "cpd_live_"+device.KeyPrefix+"_nope")
	require.Error(t, err)
	require.Equal(t, 0, repo.touchCount(), "failure must not touch last_used_at")

	authed, err := svc.Authenticate(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, 1, repo.touchCount())
	require.NotNil(t, authed.LastUsedAt)
	require.WithinDuration(t, time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC), *authed.LastUsedAt, time.Second)
}

func TestAuthenticate_TouchFailureDoesNotFailAuth(t *testing.T) {
	svc, repo, userID := newTestService(t)
	repo.touchErr = errors.New("db down")
	_, raw := repo.seed(userID, "Booth", nil)

	_, err := svc.Authenticate(context.Background(), raw)
	require.NoError(t, err)
}

func TestService_RepositoryErrorsBecomeInternal(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	dbErr := errors.New("db down")

	t.Run("create", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		repo.createErr = dbErr
		_, _, err := svc.Create(ctx, userID, CreateParams{Name: "Booth"})
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("list", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		repo.listErr = dbErr
		_, err := svc.List(ctx, userID)
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("rename", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		device, _ := repo.seed(userID, "Booth", nil)
		repo.renameErr = dbErr
		_, err := svc.Rename(ctx, userID, device.ID, "New")
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("rotate lookup", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		device, _ := repo.seed(userID, "Booth", nil)
		repo.getErr = dbErr
		_, _, err := svc.Rotate(ctx, userID, device.ID)
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("rotate update", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		device, _ := repo.seed(userID, "Booth", nil)
		repo.rotateErr = dbErr
		_, _, err := svc.Rotate(ctx, userID, device.ID)
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("revoke", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		device, _ := repo.seed(userID, "Booth", nil)
		repo.revokeErr = dbErr
		err := svc.Revoke(ctx, userID, device.ID)
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})

	t.Run("authenticate", func(t *testing.T) {
		svc, repo, _ := newTestService(t)
		_, raw := repo.seed(userID, "Booth", nil)
		repo.prefixErr = dbErr
		_, err := svc.Authenticate(ctx, raw)
		require.Equal(t, "INTERNAL_ERROR", appErrCode(t, err))
	})
}

func mustAuth(t *testing.T, svc *Service, raw string) *Device {
	t.Helper()
	d, err := svc.Authenticate(context.Background(), raw)
	require.NoError(t, err)
	return d
}
