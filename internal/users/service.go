package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetProfile(ctx context.Context, id uuid.UUID) (*User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if err == ErrNotFound {
			return nil, apperr.NotFound().WithCause(err)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return u, nil
}

func (s *Service) UpdateBusinessName(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	if len(name) > 255 {
		return nil, apperr.New("VALIDATION_ERROR", "Business name must be 255 characters or fewer", 422)
	}
	u, err := s.repo.UpdateBusinessName(ctx, id, name)
	if err != nil {
		if err == ErrNotFound {
			return nil, apperr.NotFound().WithCause(err)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return u, nil
}
