package exports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

func statusOf(t *testing.T, err error) *apperr.Error {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr
}

func newTestService(t *testing.T) (*Service, *fakeRepo, *fakeQueue, *fakePresigner) {
	t.Helper()
	repo := newFakeRepo()
	queue := &fakeQueue{}
	presigner := &fakePresigner{}
	svc := NewService(repo, fakeEvents{owned: true}, fakeCounter{count: 3}, queue, presigner, 24*time.Hour)
	svc.SetClock(func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) })
	return svc, repo, queue, presigner
}

func TestStatusValidAndActive(t *testing.T) {
	for _, s := range []Status{StatusPending, StatusProcessing, StatusReady, StatusFailed, StatusExpired} {
		require.True(t, s.Valid(), "%s should be valid", s)
	}
	require.False(t, Status("bogus").Valid())
	require.True(t, StatusPending.Active())
	require.True(t, StatusProcessing.Active())
	require.False(t, StatusReady.Active())
	require.False(t, StatusExpired.Active())
}

func TestService_CreateRejectsUnownedEvent(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	svc := NewService(repo, fakeEvents{owned: false}, fakeCounter{count: 3}, queue, &fakePresigner{}, time.Hour)

	_, err := svc.Create(context.Background(), uuid.New(), uuid.New())
	appErr := statusOf(t, err)
	require.Equal(t, "EVENT_NOT_FOUND", appErr.Code)
	require.Equal(t, 404, appErr.HTTPStatus)
	require.Zero(t, queue.count())
}

func TestService_CreateRejectsEmptyEvent(t *testing.T) {
	repo := newFakeRepo()
	queue := &fakeQueue{}
	svc := NewService(repo, fakeEvents{owned: true}, fakeCounter{count: 0}, queue, &fakePresigner{}, time.Hour)

	_, err := svc.Create(context.Background(), uuid.New(), uuid.New())
	appErr := statusOf(t, err)
	require.Equal(t, "VALIDATION_ERROR", appErr.Code)
	require.Equal(t, 422, appErr.HTTPStatus)
	require.Equal(t, "Event has no photos to export", appErr.Message)
	require.Zero(t, queue.count())
}

func TestService_CreateReusesActiveExport(t *testing.T) {
	svc, repo, queue, _ := newTestService(t)
	eventID := uuid.New()
	existing := repo.seed(&Export{
		ID:      uuid.New(),
		EventID: eventID,
		Status:  StatusProcessing,
	})

	view, err := svc.Create(context.Background(), uuid.New(), eventID)
	require.NoError(t, err)
	require.Equal(t, existing.ID, view.Export.ID)
	require.Equal(t, StatusProcessing, view.Export.Status)
	require.Zero(t, queue.count(), "active export must not queue a second job")
}

func TestService_CreateEnqueuesPendingExport(t *testing.T) {
	svc, repo, queue, _ := newTestService(t)
	userID, eventID := uuid.New(), uuid.New()

	view, err := svc.Create(context.Background(), userID, eventID)
	require.NoError(t, err)
	require.Equal(t, StatusPending, view.Export.Status)
	require.Equal(t, eventID, view.Export.EventID)
	require.Nil(t, view.DownloadURL)
	require.Equal(t, 1, queue.count())
	require.Equal(t, view.Export.ID, queue.payloads[0].ExportID)
	require.Equal(t, eventID, queue.payloads[0].EventID)

	stored, err := repo.GetByID(context.Background(), view.Export.ID)
	require.NoError(t, err)
	require.Equal(t, StatusPending, stored.Status)
}

func TestService_CreateEnqueueFailureMarksFailed(t *testing.T) {
	svc, repo, queue, _ := newTestService(t)
	queue.err = errors.New("queue down")

	_, err := svc.Create(context.Background(), uuid.New(), uuid.New())
	appErr := statusOf(t, err)
	require.Equal(t, "INTERNAL_ERROR", appErr.Code)

	require.Len(t, repo.markFailed, 1)
	stored, getErr := repo.GetByID(context.Background(), repo.markFailed[0].ID)
	require.NoError(t, getErr)
	require.Equal(t, StatusFailed, stored.Status)
}

func TestService_GetTenantIsolation(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	eventID := uuid.New()
	owner := uuid.New()
	export := repo.seed(&Export{ID: uuid.New(), EventID: eventID, Status: StatusPending, ObjectKey: nil})
	repo.owners[eventID] = owner

	_, err := svc.Get(context.Background(), uuid.New(), eventID, export.ID)
	appErr := statusOf(t, err)
	require.Equal(t, "EXPORT_NOT_FOUND", appErr.Code)
	require.Equal(t, 404, appErr.HTTPStatus)

	view, err := svc.Get(context.Background(), owner, eventID, export.ID)
	require.NoError(t, err)
	require.Equal(t, export.ID, view.Export.ID)
}

func TestService_GetDownloadURLOnlyWhenReadyAndNotExpired(t *testing.T) {
	svc, repo, _, presigner := newTestService(t)
	clock := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	eventID := uuid.New()
	key := "tenant/x/exports/e.zip"

	cases := []struct {
		name       string
		status     Status
		expiresAt  *time.Time
		wantURL    bool
		wantTTLCap time.Duration
	}{
		{name: "ready with one hour left", status: StatusReady, expiresAt: timePtr(clock.Add(time.Hour)), wantURL: true, wantTTLCap: time.Hour},
		{name: "ready longer than ttl", status: StatusReady, expiresAt: timePtr(clock.Add(48 * time.Hour)), wantURL: true, wantTTLCap: 24 * time.Hour},
		{name: "ready expired", status: StatusReady, expiresAt: timePtr(clock.Add(-time.Second)), wantURL: false},
		{name: "ready without expiry", status: StatusReady, expiresAt: nil, wantURL: false},
		{name: "processing", status: StatusProcessing, expiresAt: timePtr(clock.Add(time.Hour)), wantURL: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			export := repo.seed(&Export{
				ID:        uuid.New(),
				EventID:   eventID,
				Status:    tc.status,
				ObjectKey: &key,
				ExpiresAt: tc.expiresAt,
			})
			view, err := svc.Get(context.Background(), uuid.New(), eventID, export.ID)
			require.NoError(t, err)
			if tc.wantURL {
				require.NotNil(t, view.DownloadURL)
				require.NotEmpty(t, *view.DownloadURL)
			} else {
				require.Nil(t, view.DownloadURL)
			}
		})
	}

	require.Len(t, presigner.ttls, 2)
	require.Equal(t, time.Hour, presigner.ttls[0])
	require.Equal(t, 24*time.Hour, presigner.ttls[1])
	require.Equal(t, key, presigner.keys[0])
}

func TestService_GetPresignFailureIsInternal(t *testing.T) {
	svc, repo, _, presigner := newTestService(t)
	presigner.err = errors.New("presign down")
	eventID := uuid.New()
	key := "k"
	expires := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
	export := repo.seed(&Export{
		ID: exportIDForTest(), EventID: eventID, Status: StatusReady,
		ObjectKey: &key, ExpiresAt: &expires,
	})

	_, err := svc.Get(context.Background(), uuid.New(), eventID, export.ID)
	appErr := statusOf(t, err)
	require.Equal(t, "INTERNAL_ERROR", appErr.Code)
}

func timePtr(t time.Time) *time.Time { return &t }

func exportIDForTest() uuid.UUID { return uuid.New() }
