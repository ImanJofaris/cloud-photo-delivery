package photos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

// CleanupPayload identifies an object to delete (used when a photo fails
// permanently and its original should be removed).
type CleanupPayload struct {
	PhotoID uuid.UUID `json:"photoId"`
	EventID uuid.UUID `json:"eventId"`
}

// Deleter is the subset of object storage the cleanup handler needs.
type Deleter interface {
	Delete(ctx context.Context, key string) error
}

// CleanupHandler returns a jobs.Handler that deletes the original object of a
// photo. It is idempotent and succeeds if the object is already gone.
func CleanupHandler(repo Repository, store Deleter, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, payload []byte) error {
		var in CleanupPayload
		if err := json.Unmarshal(payload, &in); err != nil {
			return fmt.Errorf("decode cleanup payload: %w", err)
		}
		if in.PhotoID == uuid.Nil {
			return errors.New("cleanup payload missing photoId")
		}
		photo, err := repo.GetByID(ctx, in.PhotoID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return err
		}
		if err := store.Delete(ctx, photo.StorageKey); err != nil && !errors.Is(err, r2.ErrNotFound) {
			return fmt.Errorf("delete original: %w", err)
		}
		log.Info("cleaned up original", "photo_id", photo.ID)
		return nil
	}
}
