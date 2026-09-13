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

// CleanupPayload identifies objects to delete. With Keys set it deletes the
// listed objects without needing the photo row (used by photo deletion); with
// no Keys it removes the original of a still-present photo (used when
// processing fails permanently).
type CleanupPayload struct {
	PhotoID uuid.UUID `json:"photoId"`
	EventID uuid.UUID `json:"eventId"`
	Keys    []string  `json:"keys,omitempty"`
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
		if len(in.Keys) > 0 {
			for _, key := range in.Keys {
				if key == "" {
					continue
				}
				if err := store.Delete(ctx, key); err != nil && !errors.Is(err, r2.ErrNotFound) {
					return fmt.Errorf("delete object: %w", err)
				}
			}
			log.Info("cleaned up photo objects", "photo_id", in.PhotoID, "objects", len(in.Keys))
			return nil
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
