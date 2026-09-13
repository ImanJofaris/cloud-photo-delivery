package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type recordedView struct {
	eventID uuid.UUID
	day     time.Time
	hash    string
	qrScan  bool
}

type fakeRepo struct {
	views     []recordedView
	downloads []time.Time

	summary      EventReport
	daily        []DayCounters
	account      AccountReport
	accountDaily []DayCounters

	eventID      uuid.UUID
	since        time.Time
	accountSince time.Time

	recordErr       error
	summaryErr      error
	dailyErr        error
	accountErr      error
	accountDailyErr error
}

func (f *fakeRepo) RecordView(_ context.Context, eventID uuid.UUID, day time.Time, visitorHash string, qrScan bool) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.views = append(f.views, recordedView{eventID: eventID, day: day, hash: visitorHash, qrScan: qrScan})
	return nil
}

func (f *fakeRepo) RecordDownload(_ context.Context, _ uuid.UUID, day time.Time) error {
	if f.recordErr != nil {
		return f.recordErr
	}
	f.downloads = append(f.downloads, day)
	return nil
}

func (f *fakeRepo) EventSummary(_ context.Context, eventID uuid.UUID) (EventReport, error) {
	f.eventID = eventID
	if f.summaryErr != nil {
		return EventReport{}, f.summaryErr
	}
	return f.summary, nil
}

func (f *fakeRepo) EventDaily(_ context.Context, _ uuid.UUID, since time.Time) ([]DayCounters, error) {
	f.since = since
	if f.dailyErr != nil {
		return nil, f.dailyErr
	}
	return f.daily, nil
}

func (f *fakeRepo) AccountSummary(_ context.Context, _ uuid.UUID) (AccountReport, error) {
	if f.accountErr != nil {
		return AccountReport{}, f.accountErr
	}
	return f.account, nil
}

func (f *fakeRepo) AccountDaily(_ context.Context, _ uuid.UUID, since time.Time) ([]DayCounters, error) {
	f.accountSince = since
	if f.accountDailyErr != nil {
		return nil, f.accountDailyErr
	}
	return f.accountDaily, nil
}

type fakeEvents struct {
	owned      bool
	err        error
	gotUserID  uuid.UUID
	gotEventID uuid.UUID
}

func (f *fakeEvents) EventOwnedBy(_ context.Context, userID, eventID uuid.UUID) (bool, error) {
	f.gotUserID = userID
	f.gotEventID = eventID
	return f.owned, f.err
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}
