package uploads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const (
	MaxFileSize    = 100 * 1024 * 1024
	MultipartFloor = 10 * 1024 * 1024
	PartSize       = 10 * 1024 * 1024
	presignTTL     = 15 * time.Minute
	maxParts       = 10000
)

var allowedMimeTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type Clock func() time.Time

type JobQueue interface {
	EnqueueProcessPhoto(ctx context.Context, photoID, eventID uuid.UUID) error
}

// Actor is the authenticated caller: an operator (JWT) or a device key.
type Actor struct {
	UserID        uuid.UUID
	DeviceID      uuid.UUID
	AssignedEvent *uuid.UUID
}

func (a Actor) IsDevice() bool { return a.DeviceID != uuid.Nil }

type InitParams struct {
	EventID        uuid.UUID
	Filename       string
	ContentType    string
	Size           int64
	IdempotencyKey string
}

type InitResult struct {
	PhotoID    uuid.UUID
	UploadKind photos.UploadKind
	UploadURL  string
	StorageKey string
	ExpiresAt  time.Time
	PartSize   int64
}

type PartURL struct {
	PartNumber int
	URL        string
}

type PartsResult struct {
	PhotoID  uuid.UUID
	PartSize int64
	Parts    []PartURL
}

// PresignResult is a fresh presigned PUT URL for an interrupted upload.
type PresignResult struct {
	UploadURL string
	ExpiresAt time.Time
}

// Observer receives upload outcome metrics. It must be safe for concurrent
// use; nil observers are ignored.
type Observer interface {
	RecordUpload(outcome string)
}

// Upload outcomes reported to an Observer.
const (
	OutcomeInitialized = "initialized"
	OutcomeCompleted   = "completed"
	OutcomeFailed      = "failed"
)

type Service struct {
	photos   photos.Repository
	repo     Repository
	store    r2.ObjectStore
	queue    JobQueue
	limits   limits.PlanLimits
	now      Clock
	observer Observer
}

func NewService(photosRepo photos.Repository, repo Repository, store r2.ObjectStore, queue JobQueue, planLimits limits.PlanLimits) *Service {
	if queue == nil {
		queue = noopQueue{}
	}
	if planLimits == nil {
		planLimits = limits.NewDefault()
	}
	return &Service{photos: photosRepo, repo: repo, store: store, queue: queue, limits: planLimits, now: time.Now}
}

func (s *Service) SetClock(now Clock) { s.now = now }

// WithObserver attaches an upload metrics observer. Nil is a no-op.
func (s *Service) WithObserver(obs Observer) *Service {
	s.observer = obs
	return s
}

func (s *Service) observeUpload(outcome string) {
	if s.observer != nil {
		s.observer.RecordUpload(outcome)
	}
}

func (s *Service) observeResult(err error) {
	if err != nil {
		s.observeUpload(OutcomeFailed)
		return
	}
	s.observeUpload(OutcomeCompleted)
}

type noopQueue struct{}

func (noopQueue) EnqueueProcessPhoto(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func notFound() *apperr.Error {
	return apperr.New("UPLOAD_NOT_FOUND", "Upload not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func conflictError(code, msg string) *apperr.Error {
	return apperr.New(code, msg, 409)
}

func (s *Service) Initialize(ctx context.Context, actor Actor, p InitParams) (*InitResult, error) {
	result, err := s.initialize(ctx, actor, p)
	if err != nil {
		s.observeUpload(OutcomeFailed)
	} else {
		s.observeUpload(OutcomeInitialized)
	}
	return result, err
}

func (s *Service) initialize(ctx context.Context, actor Actor, p InitParams) (*InitResult, error) {
	owned, err := s.actorOwnsEvent(ctx, actor, p.EventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
	}
	if err := s.enforceEventOpen(ctx, actor.UserID, p.EventID); err != nil {
		return nil, err
	}

	filename := strings.TrimSpace(p.Filename)
	if err := validateUpload(filename, p.ContentType, p.Size); err != nil {
		return nil, err
	}
	if err := s.enforcePlanLimits(ctx, actor.UserID, p.EventID, p.Size); err != nil {
		return nil, err
	}

	photoID := uuid.New()
	storageKey := r2.OriginalKey(actor.UserID, p.EventID, photoID, filename)
	kind := photos.KindSimple
	if p.Size >= MultipartFloor {
		kind = photos.KindMultipart
	}

	if p.IdempotencyKey != "" {
		hash := requestHash(p.EventID, filename, p.ContentType, p.Size)
		if existing, err := s.repo.LookupIdempotency(ctx, p.IdempotencyKey); err == nil {
			if existing.RequestHash != hash {
				return nil, conflictError("IDEMPOTENCY_CONFLICT", "Idempotency-Key was reused with a different request")
			}
			return s.rebuildInit(ctx, actor.UserID, existing.PhotoID, kind)
		} else if !errors.Is(err, ErrNotFound) {
			return nil, apperr.Internal().WithCause(err)
		}
	}

	var multipartID *string
	if kind == photos.KindMultipart {
		id, err := s.store.CreateMultipartUpload(ctx, storageKey, p.ContentType)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		multipartID = &id
	}

	photo, err := s.photos.Create(ctx, photos.CreateInput{
		EventID:           p.EventID,
		StorageKey:        storageKey,
		OriginalFilename:  filename,
		MimeType:          p.ContentType,
		FileSize:          p.Size,
		Status:            photos.StatusUploading,
		UploadKind:        kind,
		MultipartUploadID: multipartID,
	})
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	if p.IdempotencyKey != "" {
		err := s.repo.SaveIdempotency(ctx, p.IdempotencyKey, IdempotencyRecord{
			UserID:      actor.UserID,
			PhotoID:     photo.ID,
			RequestHash: requestHash(p.EventID, filename, p.ContentType, p.Size),
		})
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
	}

	return s.buildInitResult(ctx, photo, kind)
}

func (s *Service) buildInitResult(ctx context.Context, photo *photos.Photo, kind photos.UploadKind) (*InitResult, error) {
	result := &InitResult{
		PhotoID:    photo.ID,
		UploadKind: kind,
		StorageKey: photo.StorageKey,
		ExpiresAt:  s.now().Add(presignTTL),
	}
	if kind == photos.KindMultipart {
		uploadID := ""
		if photo.MultipartUploadID != nil {
			uploadID = *photo.MultipartUploadID
		}
		url, err := s.store.PresignUploadPart(ctx, photo.StorageKey, uploadID, 1, presignTTL)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		result.UploadURL = url
		result.PartSize = PartSize
		return result, nil
	}

	url, err := s.store.PresignPut(ctx, photo.StorageKey, photo.MimeType, presignTTL)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	result.UploadURL = url
	return result, nil
}

func (s *Service) rebuildInit(ctx context.Context, userID, photoID uuid.UUID, kind photos.UploadKind) (*InitResult, error) {
	photo, err := s.photos.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, photos.ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return s.buildInitResult(ctx, photo, kind)
}

// RePresign issues a new presigned PUT for a simple upload that has not been
// completed, so a client can resume after the original URL expired.
func (s *Service) RePresign(ctx context.Context, actor Actor, photoID uuid.UUID) (*PresignResult, error) {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return nil, err
	}
	if photo.UploadKind != photos.KindSimple {
		return nil, conflictError("NOT_SIMPLE", "Photo is not a simple upload")
	}
	if photo.Status != photos.StatusUploading {
		return nil, conflictError("INVALID_UPLOAD_STATE", "Photo is no longer awaiting upload")
	}
	url, err := s.store.PresignPut(ctx, photo.StorageKey, photo.MimeType, presignTTL)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return &PresignResult{UploadURL: url, ExpiresAt: s.now().Add(presignTTL)}, nil
}

func (s *Service) Parts(ctx context.Context, actor Actor, photoID uuid.UUID, partNumbers []int) (*PartsResult, error) {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return nil, err
	}
	if photo.UploadKind != photos.KindMultipart {
		return nil, conflictError("NOT_MULTIPART", "Photo is not a multipart upload")
	}
	if len(partNumbers) == 0 {
		return nil, validationError("At least one part number is required")
	}

	uploadID := ""
	if photo.MultipartUploadID != nil {
		uploadID = *photo.MultipartUploadID
	}
	out := make([]PartURL, 0, len(partNumbers))
	for _, n := range partNumbers {
		if n < 1 || n > maxParts {
			return nil, validationError("Part number out of range")
		}
		url, err := s.store.PresignUploadPart(ctx, photo.StorageKey, uploadID, n, presignTTL)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		out = append(out, PartURL{PartNumber: n, URL: url})
	}
	return &PartsResult{PhotoID: photo.ID, PartSize: PartSize, Parts: out}, nil
}

type CompletedPart struct {
	PartNumber int
	ETag       string
}

func (s *Service) CompleteMultipart(ctx context.Context, actor Actor, photoID uuid.UUID, parts []CompletedPart) error {
	err := s.completeMultipart(ctx, actor, photoID, parts)
	s.observeResult(err)
	return err
}

func (s *Service) completeMultipart(ctx context.Context, actor Actor, photoID uuid.UUID, parts []CompletedPart) error {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return err
	}
	if photo.UploadKind != photos.KindMultipart {
		return conflictError("NOT_MULTIPART", "Photo is not a multipart upload")
	}
	if len(parts) == 0 {
		return validationError("At least one completed part is required")
	}

	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	r2parts := make([]r2.CompletePart, 0, len(parts))
	for _, p := range parts {
		if p.PartNumber < 1 || p.PartNumber > maxParts {
			return validationError("Part number out of range")
		}
		if strings.TrimSpace(p.ETag) == "" {
			return validationError("Part ETag is required")
		}
		if err := s.photos.SavePartETag(ctx, photo.ID, p.PartNumber, p.ETag); err != nil {
			return apperr.Internal().WithCause(err)
		}
		r2parts = append(r2parts, r2.CompletePart{PartNumber: p.PartNumber, ETag: p.ETag})
	}

	uploadID := ""
	if photo.MultipartUploadID != nil {
		uploadID = *photo.MultipartUploadID
	}
	if err := s.store.CompleteMultipartUpload(ctx, photo.StorageKey, uploadID, r2parts); err != nil {
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) AbortMultipart(ctx context.Context, actor Actor, photoID uuid.UUID) error {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return err
	}
	if photo.UploadKind != photos.KindMultipart {
		return conflictError("NOT_MULTIPART", "Photo is not a multipart upload")
	}
	uploadID := ""
	if photo.MultipartUploadID != nil {
		uploadID = *photo.MultipartUploadID
	}
	if err := s.store.AbortMultipartUpload(ctx, photo.StorageKey, uploadID); err != nil {
		return apperr.Internal().WithCause(err)
	}
	if err := s.photos.MarkFailed(ctx, photo.ID, "upload aborted"); err != nil && !errors.Is(err, photos.ErrNotFound) {
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) Complete(ctx context.Context, actor Actor, photoID uuid.UUID, idempotencyKey string) (*photos.Photo, error) {
	photo, err := s.complete(ctx, actor, photoID, idempotencyKey)
	s.observeResult(err)
	return photo, err
}

func (s *Service) complete(ctx context.Context, actor Actor, photoID uuid.UUID, idempotencyKey string) (*photos.Photo, error) {
	photo, err := s.photoForActor(ctx, actor, photoID)
	if err != nil {
		return nil, err
	}

	if idempotencyKey != "" {
		hash := requestHash(photoID, "complete")
		if existing, err := s.repo.LookupIdempotency(ctx, idempotencyKey); err == nil {
			if existing.RequestHash != hash || existing.PhotoID != photoID {
				return nil, conflictError("IDEMPOTENCY_CONFLICT", "Idempotency-Key was reused with a different request")
			}
		} else if errors.Is(err, ErrNotFound) {
			if err := s.repo.SaveIdempotency(ctx, idempotencyKey, IdempotencyRecord{
				UserID: actor.UserID, PhotoID: photoID, RequestHash: hash,
			}); err != nil {
				return nil, apperr.Internal().WithCause(err)
			}
		} else {
			return nil, apperr.Internal().WithCause(err)
		}
	}

	if photo.Status == photos.StatusReady || photo.Status == photos.StatusProcessing {
		return photo, nil
	}

	size, err := s.store.Head(ctx, photo.StorageKey)
	if err != nil {
		if errors.Is(err, r2.ErrNotFound) {
			return nil, apperr.New("UPLOAD_NOT_FOUND", "Uploaded object was not found", 404)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if size != photo.FileSize {
		return nil, apperr.New("UPLOAD_SIZE_MISMATCH", "Uploaded object size does not match", 422)
	}

	updated, transitioned, err := s.photos.MarkProcessing(ctx, photo.ID, size)
	if err != nil {
		if errors.Is(err, photos.ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}

	if transitioned {
		if err := s.queue.EnqueueProcessPhoto(ctx, updated.ID, updated.EventID); err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
	}
	return updated, nil
}

func (s *Service) Status(ctx context.Context, actor Actor, photoID uuid.UUID) (*photos.Photo, error) {
	return s.photoForActor(ctx, actor, photoID)
}

func (s *Service) photoForActor(ctx context.Context, actor Actor, photoID uuid.UUID) (*photos.Photo, error) {
	photo, err := s.photos.GetByID(ctx, photoID)
	if err != nil {
		if errors.Is(err, photos.ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	owned, err := s.actorOwnsEvent(ctx, actor, photo.EventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, notFound()
	}
	return photo, nil
}

// actorOwnsEvent is true when an operator owns the event or a device is
// assigned to it. Devices without an assignment cannot access any event.
func (s *Service) actorOwnsEvent(ctx context.Context, actor Actor, eventID uuid.UUID) (bool, error) {
	if actor.IsDevice() {
		return actor.AssignedEvent != nil && *actor.AssignedEvent == eventID, nil
	}
	return s.repo.EventOwnedBy(ctx, actor.UserID, eventID)
}

// enforceEventOpen rejects new uploads once an event is archived or past its
// expiry. Completion of already-initialized uploads is unaffected.
func (s *Service) enforceEventOpen(ctx context.Context, userID, eventID uuid.UUID) error {
	state, err := s.repo.EventState(ctx, userID, eventID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
		}
		return apperr.Internal().WithCause(err)
	}
	if state.Status == string(events.StatusArchived) {
		return apperr.New("INVALID_STATUS_TRANSITION", "Event is archived; uploads are closed", 409)
	}
	if state.Status == string(events.StatusExpired) ||
		(state.ExpiresAt != nil && !state.ExpiresAt.After(s.now().UTC())) {
		return apperr.New("INVALID_STATUS_TRANSITION", "Event has expired; uploads are closed", 409)
	}
	return nil
}

func (s *Service) enforcePlanLimits(ctx context.Context, userID, eventID uuid.UUID, size int64) error {
	maxPhotos, err := s.limits.MaxPhotosPerEvent(ctx, userID.String())
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	if maxPhotos > 0 {
		count, err := s.photos.CountByEvent(ctx, eventID)
		if err != nil {
			return apperr.Internal().WithCause(err)
		}
		if count >= int64(maxPhotos) {
			return apperr.New("PLAN_LIMIT_REACHED", "photosPerEvent limit reached for your plan", 402)
		}
	}

	maxBytes, err := s.limits.MaxStorageBytes(ctx, userID.String())
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	if maxBytes > 0 {
		used, err := s.repo.UserStorageBytes(ctx, userID)
		if err != nil {
			return apperr.Internal().WithCause(err)
		}
		if used+size > maxBytes {
			return apperr.New("PLAN_LIMIT_REACHED", "storageBytes limit reached for your plan", 402)
		}
	}
	return nil
}

func validateUpload(filename, contentType string, size int64) *apperr.Error {
	if strings.TrimSpace(filename) == "" {
		return validationError("Filename is required")
	}
	if len(filename) > 255 {
		return validationError("Filename must be 255 characters or fewer")
	}
	if !allowedMimeTypes[contentType] {
		return validationError("Unsupported file type")
	}
	if size <= 0 {
		return validationError("File size must be greater than zero")
	}
	if size > MaxFileSize {
		return validationError("File exceeds the 100 MB limit")
	}
	return nil
}

func requestHash(parts ...any) string {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%v|", p)
	}
	return hex.EncodeToString(h.Sum(nil))
}
