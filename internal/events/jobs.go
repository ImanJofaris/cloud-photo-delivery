package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const (
	JobEventExpire = "event.expire"
	JobEventPurge  = "event.purge"
)

const (
	expiryBatchSize = 500
	purgeBatchSize  = 100
)

// Notifier delivers expiry warnings to operators. A real mailer replaces the
// log-based implementation when email is wired in.
type Notifier interface {
	NotifyExpiryWarning(ctx context.Context, w ExpiryWarning) error
}

type LogNotifier struct {
	Log *slog.Logger
}

func (n LogNotifier) NotifyExpiryWarning(ctx context.Context, w ExpiryWarning) error {
	if n.Log == nil {
		return nil
	}
	n.Log.Info("event expiry warning", "event_id", w.EventID, "user_id", w.UserID,
		"event", w.Name, "expires_at", w.ExpiresAt.UTC())
	return nil
}

// Deleter is the subset of object storage the purge handler needs.
type Deleter interface {
	Delete(ctx context.Context, key string) error
}

// ExpireHandler marks events whose expires_at has passed as expired and emits
// at most one warning per event inside warnWithin. Warnings are best-effort:
// a delivery failure is retried on the next run and never blocks expiry.
func ExpireHandler(repo Repository, notifier Notifier, warnWithin time.Duration, now func() time.Time, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, _ []byte) error {
		at := now().UTC()
		var warnErr error
		if notifier != nil {
			warnings, err := repo.ListExpiryWarnings(ctx, at, at.Add(warnWithin), expiryBatchSize)
			if err != nil {
				return fmt.Errorf("list expiry warnings: %w", err)
			}
			for _, w := range warnings {
				if err := notifier.NotifyExpiryWarning(ctx, w); err != nil {
					warnErr = fmt.Errorf("notify expiry warning: %w", err)
					log.Error("expiry warning failed", "event_id", w.EventID, "error", err)
					continue
				}
				if err := repo.MarkExpiryWarned(ctx, w.EventID, at); err != nil {
					return fmt.Errorf("mark expiry warned: %w", err)
				}
			}
		}

		expired := 0
		for {
			due, err := repo.ListDueExpiry(ctx, at, expiryBatchSize)
			if err != nil {
				return fmt.Errorf("list due expiry: %w", err)
			}
			for _, e := range due {
				if err := repo.MarkExpired(ctx, e.ID); err != nil {
					return fmt.Errorf("mark expired: %w", err)
				}
				expired++
			}
			if len(due) < expiryBatchSize {
				break
			}
		}
		if expired > 0 {
			log.Info("events expired", "count", expired)
		}
		return warnErr
	}
}

// PurgeHandler permanently deletes soft-deleted and expired events that are
// past the grace period. Objects are removed before rows, so a crash between
// the two is retried from the still-present database state.
func PurgeHandler(repo Repository, store Deleter, grace time.Duration, now func() time.Time, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, _ []byte) error {
		cutoff := now().UTC().Add(-grace)
		purged := 0
		for {
			candidates, err := repo.ListPurgeable(ctx, cutoff, purgeBatchSize)
			if err != nil {
				return fmt.Errorf("list purgeable events: %w", err)
			}
			for _, c := range candidates {
				if err := purgeEvent(ctx, repo, store, c, log); err != nil {
					return err
				}
				purged++
			}
			if len(candidates) < purgeBatchSize {
				break
			}
		}
		if purged > 0 {
			log.Info("events purged", "count", purged)
		}
		return nil
	}
}

func purgeEvent(ctx context.Context, repo Repository, store Deleter, c PurgeCandidate, log *slog.Logger) error {
	keys, err := repo.PurgeKeys(ctx, c.EventID)
	if err != nil {
		return fmt.Errorf("list purge keys: %w", err)
	}
	for _, key := range keys {
		if err := store.Delete(ctx, key); err != nil && !errors.Is(err, r2.ErrNotFound) {
			return fmt.Errorf("delete object: %w", err)
		}
	}
	deleted, err := repo.PurgeEvent(ctx, c.EventID)
	if err != nil {
		return fmt.Errorf("purge event: %w", err)
	}
	if deleted {
		log.Info("event purged", "event_id", c.EventID, "user_id", c.UserID,
			"objects", len(keys), "storage_bytes", c.StorageBytes)
	}
	return nil
}
