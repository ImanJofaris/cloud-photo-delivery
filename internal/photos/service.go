package photos

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

// CleanupQueue enqueues object removal jobs after a photo is deleted.
type CleanupQueue interface {
	EnqueueObjectCleanup(ctx context.Context, payload CleanupPayload) error
}

// MultipartAborter is the object-store subset used to cancel an in-progress
// multipart upload when its photo is deleted.
type MultipartAborter interface {
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}

type Service struct {
	repo  Repository
	urls  *SignedURLGenerator
	store MultipartAborter
	queue CleanupQueue
}

func NewService(repo Repository, urls *SignedURLGenerator, store MultipartAborter, queue CleanupQueue) *Service {
	return &Service{repo: repo, urls: urls, store: store, queue: queue}
}

type ListResult struct {
	Items      []*Photo
	NextCursor string
}

func notFound() *apperr.Error {
	return apperr.New("PHOTO_NOT_FOUND", "Photo not found", 404)
}

func eventNotFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

// eventOwnedBy authorizes an actor for an event: devices only for their
// assigned event, operators via an ownership query.
func (s *Service) eventOwnedBy(ctx context.Context, actor Actor, eventID uuid.UUID) (bool, error) {
	if actor.IsDevice() {
		return actor.AssignedEvent != nil && *actor.AssignedEvent == eventID, nil
	}
	return s.repo.EventOwnedBy(ctx, actor.UserID, eventID)
}

func (s *Service) photoForActor(ctx context.Context, actor Actor, photoID uuid.UUID) (*Photo, error) {
	photo, err := s.repo.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	owned, err := s.eventOwnedBy(ctx, actor, photo.EventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, notFound()
	}
	return photo, nil
}

func (s *Service) List(ctx context.Context, actor Actor, eventID uuid.UUID, cursorRaw, statusRaw string, limit int) (*ListResult, error) {
	cursor, err := DecodeCursor(cursorRaw)
	if err != nil {
		return nil, validationError("Invalid cursor")
	}
	var status Status
	if raw := strings.TrimSpace(statusRaw); raw != "" {
		status = Status(raw)
		if !status.Valid() {
			return nil, validationError("Invalid status")
		}
	}
	owned, err := s.eventOwnedBy(ctx, actor, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, eventNotFound()
	}

	n := NormalizeLimit(limit)
	items, err := s.repo.ListByEvent(ctx, ListInput{
		EventID: eventID,
		UserID:  actor.UserID,
		Cursor:  cursor,
		Status:  status,
		Limit:   n,
	})
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	result := &ListResult{Items: items}
	if len(items) == n && n > 0 {
		last := items[len(items)-1]
		result.NextCursor = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return result, nil
}

func (s *Service) URL(ctx context.Context, actor Actor, photoID uuid.UUID, variant string) (*URLResult, error) {
	v := strings.ToLower(strings.TrimSpace(variant))
	switch v {
	case "thumbnail", "medium", "large", "original":
	default:
		return nil, validationError("Invalid variant")
	}
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return nil, err
	}
	result, err := s.urls.URL(ctx, photo, v)
	if err != nil {
		if errors.Is(err, ErrVariantUnavailable) {
			return nil, apperr.New("VARIANT_UNAVAILABLE", "Requested variant is not available", 404)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return result, nil
}

func (s *Service) Delete(ctx context.Context, actor Actor, photoID uuid.UUID) error {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return err
	}

	// Best effort: an already completed or expired multipart upload must not
	// block deletion. The cleanup job still removes any stored objects.
	if photo.UploadKind == KindMultipart && photo.MultipartUploadID != nil && *photo.MultipartUploadID != "" {
		_ = s.store.AbortMultipartUpload(ctx, photo.StorageKey, *photo.MultipartUploadID)
	}

	deleted, err := s.repo.DeleteOwned(ctx, photo.ID, actor.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return notFound()
		}
		return apperr.Internal().WithCause(err)
	}

	payload := CleanupPayload{PhotoID: photo.ID, EventID: deleted.EventID, Keys: deleted.Keys}
	if err := s.queue.EnqueueObjectCleanup(ctx, payload); err != nil {
		return apperr.Internal().WithCause(err)
	}
	return nil
}
