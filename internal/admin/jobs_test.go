package admin

import (
	"context"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
)

func fixedNow() time.Time {
	return time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
}

func TestReconcileHandler_DetectsDrift(t *testing.T) {
	repo := newFakeRepo()
	repo.totals = &StorageTotals{StorageBytes: 1000, Photos: 2}
	lister := &fakeLister{objects: []r2.ObjectInfo{
		{Key: "tenant/u1/events/e1/originals/p1/a.jpg", Size: 400},
		{Key: "tenant/u1/events/e1/originals/p2/b.jpg", Size: 700},
		{Key: "tenant/u1/events/e1/thumbnails/p1.webp", Size: 50},
		{Key: "tenant/u1/events/e1/exports/z.zip", Size: 900},
		{Key: "tenant/u1/branding/logo/x.png", Size: 10},
	}}
	reporter := &fakeReporter{}

	handler := ReconcileHandler(repo, lister, fixedNow, reporter)
	require.NoError(t, handler(context.Background(), nil))

	require.Equal(t, originalsPrefix, lister.prefix)
	require.Len(t, reporter.reports, 1)
	report := reporter.reports[0]
	require.Equal(t, fixedNow(), report.CheckedAt)
	require.Equal(t, int64(1000), report.DBStorageBytes)
	require.Equal(t, int64(2), report.DBPhotos)
	require.Equal(t, int64(1100), report.R2StorageBytes)
	require.Equal(t, int64(2), report.R2Objects)
	require.Equal(t, int64(100), report.DriftBytes())
	require.Equal(t, int64(0), report.DriftObjects())
	require.False(t, report.InSync())
}

func TestReconcileHandler_InSync(t *testing.T) {
	repo := newFakeRepo()
	repo.totals = &StorageTotals{StorageBytes: 400, Photos: 1}
	lister := &fakeLister{objects: []r2.ObjectInfo{
		{Key: "tenant/u1/events/e1/originals/p1/a.jpg", Size: 400},
	}}
	reporter := &fakeReporter{}

	require.NoError(t, ReconcileHandler(repo, lister, fixedNow, reporter)(context.Background(), nil))
	require.Len(t, reporter.reports, 1)
	require.True(t, reporter.reports[0].InSync())
}

func TestReconcileHandler_NilReporterIsAllowed(t *testing.T) {
	repo := newFakeRepo()
	lister := &fakeLister{}
	require.NoError(t, ReconcileHandler(repo, lister, fixedNow, nil)(context.Background(), nil))
}

func TestReconcileHandler_Errors(t *testing.T) {
	repo := newFakeRepo()
	repo.totalsErr = errBoom
	err := ReconcileHandler(repo, &fakeLister{}, fixedNow, &fakeReporter{})(context.Background(), nil)
	require.ErrorContains(t, err, "storage totals")

	repo = newFakeRepo()
	lister := &fakeLister{err: errBoom}
	err = ReconcileHandler(repo, lister, fixedNow, &fakeReporter{})(context.Background(), nil)
	require.ErrorContains(t, err, "list objects")

	repo = newFakeRepo()
	err = ReconcileHandler(repo, &fakeLister{}, fixedNow, &fakeReporter{err: errBoom})(context.Background(), nil)
	require.ErrorContains(t, err, "report reconcile")
}

func TestIsOriginalKey(t *testing.T) {
	require.True(t, isOriginalKey("tenant/u/events/e/originals/p/a.jpg"))
	require.False(t, isOriginalKey("tenant/u/events/e/thumbnails/p.webp"))
	require.False(t, isOriginalKey("tenant/u/events/e/exports/p.zip"))
	require.False(t, isOriginalKey("tenant/u/branding/logo/x.png"))
}
