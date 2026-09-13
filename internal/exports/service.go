package exports

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type EventChecker interface {
	EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

type PhotoCounter interface {
	CountByEvent(ctx context.Context, eventID uuid.UUID) (int64, error)
}

type Queue interface {
	EnqueueGenerate(ctx context.Context, payload GeneratePayload) error
}

type Presigner interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

type Service struct {
	repo      Repository
	events    EventChecker
	photos    PhotoCounter
	queue     Queue
	presigner Presigner
	ttl       time.Duration
	now       func() time.Time
}

func NewService(repo Repository, events EventChecker, photos PhotoCounter, queue Queue, presigner Presigner, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Service{
		repo:      repo,
		events:    events,
		photos:    photos,
		queue:     queue,
		presigner: presigner,
		ttl:       ttl,
		now:       time.Now,
	}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

// View is an export plus the optional signed download URL for ready archives.
type View struct {
	Export      *Export
	DownloadURL *string
}

func eventNotFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

func exportNotFound() *apperr.Error {
	return apperr.New("EXPORT_NOT_FOUND", "Export not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

// Create authorizes the event, rejects empty events, and reuses an already
// pending/processing export instead of queueing a duplicate archive.
func (s *Service) Create(ctx context.Context, userID, eventID uuid.UUID) (*View, error) {
	owned, err := s.events.EventOwnedBy(ctx, userID, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, eventNotFound()
	}

	count, err := s.photos.CountByEvent(ctx, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if count == 0 {
		return nil, validationError("Event has no photos to export")
	}

	active, err := s.repo.GetActiveByEvent(ctx, eventID)
	if err == nil {
		return &View{Export: active}, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, apperr.Internal().WithCause(err)
	}

	export, err := s.repo.Create(ctx, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	if err := s.queue.EnqueueGenerate(ctx, GeneratePayload{ExportID: export.ID, EventID: export.EventID}); err != nil {
		_ = s.repo.MarkFailed(ctx, export.ID, "could not enqueue export job")
		return nil, apperr.Internal().WithCause(err)
	}
	return &View{Export: export}, nil
}

func (s *Service) Get(ctx context.Context, userID, eventID, exportID uuid.UUID) (*View, error) {
	export, err := s.repo.GetOwned(ctx, userID, eventID, exportID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, exportNotFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}

	view := &View{Export: export}
	url, err := s.downloadURL(ctx, export)
	if err != nil {
		return nil, err
	}
	view.DownloadURL = url
	return view, nil
}

// downloadURL signs the archive for its remaining lifetime, capped at the
// configured export TTL. An archive with under a second left is treated as
// expired: the URL is omitted and cleanup will flip its status.
func (s *Service) downloadURL(ctx context.Context, export *Export) (*string, error) {
	if export.Status != StatusReady || export.ObjectKey == nil || *export.ObjectKey == "" || export.ExpiresAt == nil {
		return nil, nil
	}
	if s.presigner == nil {
		return nil, nil
	}
	remaining := export.ExpiresAt.Sub(s.now().UTC())
	if remaining > s.ttl {
		remaining = s.ttl
	}
	if remaining < time.Second {
		return nil, nil
	}
	url, err := s.presigner.PresignGet(ctx, *export.ObjectKey, remaining)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return &url, nil
}
