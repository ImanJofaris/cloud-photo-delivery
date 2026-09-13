package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

const (
	StatusOK       = "ok"
	StatusDegraded = "degraded"
)

type Service struct {
	repo  Repository
	clock func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, clock: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) SetClock(now func() time.Time) {
	if now != nil {
		s.clock = now
	}
}

func (s *Service) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	ok, err := s.repo.IsAdmin(ctx, userID)
	if err != nil {
		return false, apperr.Internal().WithCause(err)
	}
	return ok, nil
}

func (s *Service) Stats(ctx context.Context) (*Stats, error) {
	stats, err := s.repo.Stats(ctx)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return stats, nil
}

type UserList struct {
	Users      []*UserSummary
	NextCursor string
}

func (s *Service) ListUsers(ctx context.Context, cursor string, limit int) (*UserList, error) {
	c, err := DecodeCursor(cursor)
	if err != nil {
		return nil, invalidCursor()
	}
	limit = NormalizeLimit(limit)

	users, err := s.repo.ListUsers(ctx, c, limit)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	result := &UserList{Users: users}
	if len(users) == limit && len(users) > 0 {
		last := users[len(users)-1]
		result.NextCursor = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return result, nil
}

type SubscriptionList struct {
	Subscriptions []*SubscriptionSummary
	NextCursor    string
}

func (s *Service) ListSubscriptions(ctx context.Context, cursor string, limit int) (*SubscriptionList, error) {
	c, err := DecodeCursor(cursor)
	if err != nil {
		return nil, invalidCursor()
	}
	limit = NormalizeLimit(limit)

	subs, err := s.repo.ListSubscriptions(ctx, c, limit)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	result := &SubscriptionList{Subscriptions: subs}
	if len(subs) == limit && len(subs) > 0 {
		last := subs[len(subs)-1]
		result.NextCursor = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return result, nil
}

func (s *Service) Health(ctx context.Context) (*Health, error) {
	queue, err := s.repo.QueueHealth(ctx)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	status := StatusOK
	if queue.Failed > 0 {
		status = StatusDegraded
	}
	return &Health{Status: status, Queue: *queue, CheckedAt: s.clock()}, nil
}

func invalidCursor() *apperr.Error {
	return apperr.New("VALIDATION_ERROR", "Invalid cursor", 422)
}
