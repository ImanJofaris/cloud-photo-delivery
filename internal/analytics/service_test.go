package analytics

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

func testService() (*Service, *fakeRepo, *fakeEvents) {
	repo := &fakeRepo{}
	events := &fakeEvents{owned: true}
	svc := NewService(repo, events, "test-salt", nil)
	svc.SetClock(fixedClock(time.Date(2026, 9, 13, 15, 4, 5, 0, time.UTC)))
	return svc, repo, events
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func TestService_RecordView(t *testing.T) {
	svc, repo, _ := testService()
	eventID := uuid.New()

	require.NoError(t, svc.RecordView(context.Background(), eventID, "1.2.3.4", "agent", true))
	require.Len(t, repo.views, 1)
	require.Equal(t, eventID, repo.views[0].eventID)
	require.True(t, repo.views[0].qrScan)
	require.Equal(t, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), repo.views[0].day)
	require.NotContains(t, repo.views[0].hash, "1.2.3.4")
	require.Len(t, repo.views[0].hash, 64)
}

func TestService_RecordDownload(t *testing.T) {
	svc, repo, _ := testService()
	require.NoError(t, svc.RecordDownload(context.Background(), uuid.New()))
	require.Len(t, repo.downloads, 1)
	require.Equal(t, time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC), repo.downloads[0])
}

func TestService_RecordView_ErrorIsReturned(t *testing.T) {
	svc, repo, _ := testService()
	repo.recordErr = errors.New("db down")
	require.Error(t, svc.RecordView(context.Background(), uuid.New(), "1.2.3.4", "agent", false))
}

func TestHashVisitor_DeterministicAndSalted(t *testing.T) {
	a := hashVisitor("salt-a", "1.2.3.4", "agent")
	require.Equal(t, a, hashVisitor("salt-a", "1.2.3.4", "agent"))
	require.NotEqual(t, a, hashVisitor("salt-b", "1.2.3.4", "agent"))
	require.NotEqual(t, a, hashVisitor("salt-a", "5.6.7.8", "agent"))
	require.NotEqual(t, a, hashVisitor("salt-a", "1.2.3.4", "other"))
}

func TestService_Event_NotOwned(t *testing.T) {
	svc, repo, events := testService()
	events.owned = false

	_, err := svc.Event(context.Background(), uuid.New(), uuid.New(), 0)
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
	require.Empty(t, repo.views)
}

func TestService_Event_AssemblesReport(t *testing.T) {
	svc, repo, events := testService()
	userID, eventID := uuid.New(), uuid.New()
	repo.summary = EventReport{EventID: eventID, PhotoCount: 42}
	repo.daily = []DayCounters{
		{Day: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC), Counters: Counters{GalleryViews: 3}},
	}

	report, err := svc.Event(context.Background(), userID, eventID, 7)
	require.NoError(t, err)
	require.Equal(t, int64(42), report.PhotoCount)
	require.Len(t, report.Daily, 1)
	require.Equal(t, userID, events.gotUserID)
	require.Equal(t, eventID, events.gotEventID)
	require.Equal(t, time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), repo.since, "days=7 includes today")
}

func TestService_Account_AssemblesReport(t *testing.T) {
	svc, repo, _ := testService()
	repo.account = AccountReport{EventCount: 2, PhotoCount: 80}
	repo.accountDaily = []DayCounters{{Day: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)}}

	report, err := svc.Account(context.Background(), uuid.New(), 0)
	require.NoError(t, err)
	require.Equal(t, int64(2), report.EventCount)
	require.Equal(t, int64(80), report.PhotoCount)
	require.Len(t, report.Daily, 1)
	require.Equal(t, time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), repo.accountSince, "default window is 30 days")
}

func TestService_WindowValidation(t *testing.T) {
	svc, _, _ := testService()
	for _, days := range []int{-1, -30, 366, 1000} {
		_, err := svc.Event(context.Background(), uuid.New(), uuid.New(), days)
		require.Equal(t, "VALIDATION_ERROR", codeOf(t, err), "days=%d", days)
		require.True(t, strings.Contains(err.Error(), "days"), "days=%d", days)
	}
}

func TestService_Event_RepoErrorIsInternal(t *testing.T) {
	svc, repo, _ := testService()
	repo.summaryErr = errors.New("db down")
	_, err := svc.Event(context.Background(), uuid.New(), uuid.New(), 0)
	require.Equal(t, "INTERNAL_ERROR", codeOf(t, err))
}
