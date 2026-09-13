package devices

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

const maxNameLength = 120

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

func notFound() *apperr.Error {
	return apperr.New("DEVICE_NOT_FOUND", "Device not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func invalidKey() *apperr.Error {
	return apperr.New("INVALID_DEVICE_KEY", "Invalid device key", 401)
}

func revokedKey() *apperr.Error {
	return apperr.New("DEVICE_REVOKED", "Device key has been revoked", 401)
}

type CreateParams struct {
	Name            string
	AssignedEventID *uuid.UUID
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, p CreateParams) (*Device, string, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, "", validationError("Device name is required")
	}
	if len(name) > maxNameLength {
		return nil, "", validationError("Device name must be 120 characters or fewer")
	}
	if p.AssignedEventID != nil {
		owned, err := s.repo.EventOwnedBy(ctx, userID, *p.AssignedEventID)
		if err != nil {
			return nil, "", apperr.Internal().WithCause(err)
		}
		if !owned {
			return nil, "", apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
		}
	}

	raw, prefix, hash, err := GenerateKey()
	if err != nil {
		return nil, "", apperr.Internal().WithCause(err)
	}

	created, err := s.repo.Create(ctx, &Device{
		ID:              uuid.New(),
		UserID:          userID,
		Name:            name,
		KeyPrefix:       prefix,
		KeyHash:         hash,
		AssignedEventID: p.AssignedEventID,
		CreatedAt:       s.now().UTC(),
	})
	if err != nil {
		return nil, "", apperr.Internal().WithCause(err)
	}
	return created, raw, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]*Device, error) {
	items, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return items, nil
}

func (s *Service) Rename(ctx context.Context, userID, id uuid.UUID, name string) (*Device, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, validationError("Device name is required")
	}
	if len(name) > maxNameLength {
		return nil, validationError("Device name must be 120 characters or fewer")
	}
	updated, err := s.repo.Rename(ctx, userID, id, name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Rotate(ctx context.Context, userID, id uuid.UUID) (*Device, string, error) {
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, "", notFound()
		}
		return nil, "", apperr.Internal().WithCause(err)
	}
	if current.Revoked() {
		return nil, "", apperr.New("DEVICE_REVOKED", "Device has been revoked", 409)
	}

	raw, prefix, hash, err := GenerateKey()
	if err != nil {
		return nil, "", apperr.Internal().WithCause(err)
	}
	updated, err := s.repo.Rotate(ctx, userID, id, prefix, hash)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, "", notFound()
		}
		return nil, "", apperr.Internal().WithCause(err)
	}
	return updated, raw, nil
}

func (s *Service) Revoke(ctx context.Context, userID, id uuid.UUID) error {
	if err := s.repo.Revoke(ctx, userID, id, s.now().UTC()); err != nil {
		if errors.Is(err, ErrNotFound) {
			return notFound()
		}
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) Authenticate(ctx context.Context, rawKey string) (*Device, error) {
	prefix, err := KeyPrefix(rawKey)
	if err != nil {
		return nil, invalidKey().WithCause(err)
	}
	device, err := s.repo.GetByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, invalidKey()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if !VerifyKey(rawKey, device.KeyHash) {
		return nil, invalidKey()
	}
	if device.Revoked() {
		return nil, revokedKey()
	}
	// Best-effort telemetry: a failed touch must not fail authentication.
	at := s.now().UTC()
	if err := s.repo.TouchLastUsed(ctx, device.ID, at); err == nil {
		device.LastUsedAt = &at
	}
	return device, nil
}
