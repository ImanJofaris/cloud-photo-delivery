package photos

import (
	"context"
	"log/slog"
	"time"
)

const JobStaleUploadSweep = "uploads.stale_sweep"

type staleUploadRepo interface {
	FailStaleUploads(ctx context.Context, cutoff time.Time) (int, error)
}

// StaleUploadHandler fails uploads that stopped reporting progress, so rows
// left in UPLOADING by a vanished client do not poll forever. Idempotent.
func StaleUploadHandler(repo staleUploadRepo, staleAfter time.Duration, now func() time.Time, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, _ []byte) error {
		failed, err := repo.FailStaleUploads(ctx, now().UTC().Add(-staleAfter))
		if err != nil {
			return err
		}
		if failed > 0 && log != nil {
			log.Info("stale uploads failed", "count", failed)
		}
		return nil
	}
}
