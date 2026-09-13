package admin

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

var errBoom = errors.New("boom")

type fakeRepo struct {
	stats  *Stats
	users  []*UserSummary
	subs   []*SubscriptionSummary
	queue  *QueueHealth
	totals *StorageTotals
	admins map[uuid.UUID]bool

	statsErr   error
	usersErr   error
	subsErr    error
	queueErr   error
	isAdminErr error
	totalsErr  error

	lastUserCursor *Cursor
	lastUserLimit  int
	lastSubCursor  *Cursor
	lastSubLimit   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		stats:  &Stats{},
		queue:  &QueueHealth{},
		totals: &StorageTotals{},
		admins: map[uuid.UUID]bool{},
	}
}

func (f *fakeRepo) Stats(context.Context) (*Stats, error) {
	if f.statsErr != nil {
		return nil, f.statsErr
	}
	return f.stats, nil
}

func (f *fakeRepo) ListUsers(_ context.Context, cursor *Cursor, limit int) ([]*UserSummary, error) {
	f.lastUserCursor = cursor
	f.lastUserLimit = limit
	if f.usersErr != nil {
		return nil, f.usersErr
	}
	return f.users, nil
}

func (f *fakeRepo) ListSubscriptions(_ context.Context, cursor *Cursor, limit int) ([]*SubscriptionSummary, error) {
	f.lastSubCursor = cursor
	f.lastSubLimit = limit
	if f.subsErr != nil {
		return nil, f.subsErr
	}
	return f.subs, nil
}

func (f *fakeRepo) QueueHealth(context.Context) (*QueueHealth, error) {
	if f.queueErr != nil {
		return nil, f.queueErr
	}
	return f.queue, nil
}

func (f *fakeRepo) IsAdmin(_ context.Context, userID uuid.UUID) (bool, error) {
	if f.isAdminErr != nil {
		return false, f.isAdminErr
	}
	return f.admins[userID], nil
}

func (f *fakeRepo) StorageTotals(context.Context) (*StorageTotals, error) {
	if f.totalsErr != nil {
		return nil, f.totalsErr
	}
	return f.totals, nil
}

type fakeLister struct {
	objects []r2.ObjectInfo
	err     error
	prefix  string
}

func (f *fakeLister) ListObjects(_ context.Context, prefix string) ([]r2.ObjectInfo, error) {
	f.prefix = prefix
	if f.err != nil {
		return nil, f.err
	}
	return f.objects, nil
}

type fakeReporter struct {
	reports []ReconcileReport
	err     error
}

func (f *fakeReporter) ReportReconcile(_ context.Context, report ReconcileReport) error {
	f.reports = append(f.reports, report)
	return f.err
}
