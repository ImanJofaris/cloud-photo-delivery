package admin

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const JobStorageReconcile = "storage.reconcile"

// originalsPrefix bounds a full-bucket listing to tenant objects.
const originalsPrefix = "tenant/"

// ObjectLister is the object-storage subset the reconcile job needs.
type ObjectLister interface {
	ListObjects(ctx context.Context, prefix string) ([]r2.ObjectInfo, error)
}

// ReconcileReporter receives every reconcile result. The worker wires the
// log-based implementation until an alerting sink exists.
type ReconcileReporter interface {
	ReportReconcile(ctx context.Context, report ReconcileReport) error
}

type LogReconcileReporter struct {
	Log *slog.Logger
}

func (r LogReconcileReporter) ReportReconcile(_ context.Context, report ReconcileReport) error {
	if r.Log == nil {
		return nil
	}
	if report.InSync() {
		r.Log.Info("storage reconciled",
			"db_bytes", report.DBStorageBytes, "r2_bytes", report.R2StorageBytes,
			"db_photos", report.DBPhotos, "r2_objects", report.R2Objects)
		return nil
	}
	r.Log.Warn("storage drift detected",
		"db_bytes", report.DBStorageBytes, "r2_bytes", report.R2StorageBytes,
		"drift_bytes", report.DriftBytes(), "db_photos", report.DBPhotos,
		"r2_objects", report.R2Objects, "drift_objects", report.DriftObjects())
	return nil
}

// ReconcileHandler compares the database storage counters with the original
// objects in R2 and reports the drift. It never deletes objects: purge owns
// deletion, so a drift is surfaced for an operator to investigate.
func ReconcileHandler(repo Repository, lister ObjectLister, now func() time.Time, reporter ReconcileReporter) func(context.Context, []byte) error {
	return func(ctx context.Context, _ []byte) error {
		totals, err := repo.StorageTotals(ctx)
		if err != nil {
			return fmt.Errorf("storage totals: %w", err)
		}
		objects, err := lister.ListObjects(ctx, originalsPrefix)
		if err != nil {
			return fmt.Errorf("list objects: %w", err)
		}

		var bytes, count int64
		for _, o := range objects {
			if !isOriginalKey(o.Key) {
				continue
			}
			bytes += o.Size
			count++
		}

		report := ReconcileReport{
			CheckedAt:      now().UTC(),
			DBStorageBytes: totals.StorageBytes,
			DBPhotos:       totals.Photos,
			R2StorageBytes: bytes,
			R2Objects:      count,
		}
		if reporter != nil {
			if err := reporter.ReportReconcile(ctx, report); err != nil {
				return fmt.Errorf("report reconcile: %w", err)
			}
		}
		return nil
	}
}

func isOriginalKey(key string) bool {
	return strings.Contains(key, "/originals/")
}
