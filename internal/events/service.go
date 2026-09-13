package events

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/limits"
)

const maxNameLength = 255

const maxExtendDays = 3650

type PlanLimits = limits.PlanLimits

type CreateParams struct {
	Name        string
	EventDate   *time.Time
	ClientName  string
	ClientEmail string
	Location    string
	Description string
	Status      Status
	ExpiresAt   *time.Time
	Settings    SettingsInput
}

type SettingsInput struct {
	Visibility            *Visibility
	Password              *string
	AllowDownload         *bool
	AllowOriginalDownload *bool
	WatermarkEnabled      *bool
}

type UpdateParams struct {
	Name        *string
	EventDate   *time.Time
	ClearDate   bool
	ClientName  *string
	ClientEmail *string
	Location    *string
	Description *string
	ExpiresAt   *time.Time
	ClearExpiry bool
}

type ListParams struct {
	Status *Status
	Query  string
	Cursor string
	Limit  int
}

type ListResult struct {
	Events     []*Event
	NextCursor string
}

type HashFunc func(password string) (string, error)

type Service struct {
	repo   Repository
	limits PlanLimits
	hash   HashFunc
	now    func() time.Time
}

func NewService(repo Repository, planLimits PlanLimits, hash HashFunc) *Service {
	if planLimits == nil {
		planLimits = limits.NewDefault()
	}
	if hash == nil {
		hash = func(string) (string, error) { return "", nil }
	}
	return &Service{repo: repo, limits: planLimits, hash: hash, now: time.Now}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

func notFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, p CreateParams) (*Event, *Settings, error) {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, nil, validationError("Event name is required")
	}
	if len(name) > maxNameLength {
		return nil, nil, validationError("Event name must be 255 characters or fewer")
	}
	if p.ClientEmail != "" && !validEmail(p.ClientEmail) {
		return nil, nil, validationError("Client email is invalid")
	}
	status := p.Status
	if status == "" {
		status = StatusUpcoming
	}
	if !status.Valid() {
		return nil, nil, validationError("Invalid event status")
	}

	if status == StatusActive {
		if err := s.enforceActiveLimit(ctx, userID); err != nil {
			return nil, nil, err
		}
	}

	settings, err := s.buildSettings(Settings{Visibility: VisibilityPublic, AllowDownload: true}, p.Settings)
	if err != nil {
		return nil, nil, err
	}

	slug, err := s.uniqueSlug(ctx, userID, name)
	if err != nil {
		return nil, nil, err
	}

	expiresAt, err := s.resolveExpiry(ctx, userID, p.ExpiresAt)
	if err != nil {
		return nil, nil, err
	}

	event, created, err := s.repo.Create(ctx, CreateInput{
		UserID:      userID,
		Name:        name,
		Slug:        slug,
		ClientName:  strings.TrimSpace(p.ClientName),
		ClientEmail: strings.TrimSpace(p.ClientEmail),
		Location:    strings.TrimSpace(p.Location),
		Description: p.Description,
		EventDate:   p.EventDate,
		Status:      status,
		ExpiresAt:   expiresAt,
		Settings:    settings,
	})
	if err != nil {
		return nil, nil, apperr.Internal().WithCause(err)
	}
	return event, created, nil
}

func (s *Service) resolveExpiry(ctx context.Context, userID uuid.UUID, explicit *time.Time) (*time.Time, error) {
	if explicit != nil {
		return explicit, nil
	}
	days, err := s.limits.RetentionDays(ctx, userID.String())
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if days <= 0 {
		return nil, nil
	}
	t := s.now().UTC().AddDate(0, 0, days)
	return &t, nil
}

func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (*Event, error) {
	e, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return e, nil
}

func (s *Service) List(ctx context.Context, userID uuid.UUID, p ListParams) (*ListResult, error) {
	if p.Status != nil && !p.Status.Valid() {
		return nil, validationError("Invalid status filter")
	}
	cursor, err := DecodeCursor(p.Cursor)
	if err != nil {
		return nil, validationError("Invalid cursor")
	}
	limit := NormalizeLimit(p.Limit)

	items, err := s.repo.List(ctx, userID, ListFilter{
		Status: p.Status,
		Query:  strings.TrimSpace(p.Query),
		Cursor: cursor,
		Limit:  limit,
	})
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	result := &ListResult{Events: items}
	if len(items) == limit && len(items) > 0 {
		last := items[len(items)-1]
		result.NextCursor = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, userID, id uuid.UUID, p UpdateParams) (*Event, error) {
	if p.Name != nil {
		name := strings.TrimSpace(*p.Name)
		if name == "" {
			return nil, validationError("Event name is required")
		}
		if len(name) > maxNameLength {
			return nil, validationError("Event name must be 255 characters or fewer")
		}
		p.Name = &name
	}
	if p.ClientEmail != nil && *p.ClientEmail != "" && !validEmail(*p.ClientEmail) {
		return nil, validationError("Client email is invalid")
	}

	e, err := s.repo.Update(ctx, userID, id, UpdateInput{
		Name:        p.Name,
		ClientName:  p.ClientName,
		ClientEmail: p.ClientEmail,
		Location:    p.Location,
		Description: p.Description,
		EventDate:   p.EventDate,
		ClearDate:   p.ClearDate,
		ExpiresAt:   p.ExpiresAt,
		ClearExpiry: p.ClearExpiry,
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return e, nil
}

func (s *Service) Archive(ctx context.Context, userID, id uuid.UUID) (*Event, error) {
	return s.transition(ctx, userID, id, StatusArchived)
}

func (s *Service) Extend(ctx context.Context, userID, id uuid.UUID, days int) (*Event, error) {
	if days < 1 || days > maxExtendDays {
		return nil, validationError("days must be between 1 and 3650")
	}
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if current.Status == StatusArchived {
		return nil, apperr.New("INVALID_STATUS_TRANSITION", "Cannot extend an archived event", 409)
	}
	reactivate := current.Status == StatusExpired
	if reactivate {
		if err := s.enforceActiveLimit(ctx, userID); err != nil {
			return nil, err
		}
	}
	now := s.now().UTC()
	base := now
	if current.ExpiresAt != nil && current.ExpiresAt.After(now) {
		base = *current.ExpiresAt
	}
	expiresAt := base.AddDate(0, 0, days)

	updated, err := s.repo.Extend(ctx, userID, id, expiresAt, reactivate)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Transition(ctx context.Context, userID, id uuid.UUID, to Status) (*Event, error) {
	return s.transition(ctx, userID, id, to)
}

func (s *Service) transition(ctx context.Context, userID, id uuid.UUID, to Status) (*Event, error) {
	if !to.Valid() {
		return nil, validationError("Invalid event status")
	}
	current, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if current.Status == to {
		return current, nil
	}
	if !CanTransition(current.Status, to) {
		return nil, apperr.New("INVALID_STATUS_TRANSITION",
			"Cannot transition event from "+string(current.Status)+" to "+string(to), 409)
	}
	if to == StatusActive {
		if err := s.enforceActiveLimit(ctx, userID); err != nil {
			return nil, err
		}
	}
	updated, err := s.repo.SetStatus(ctx, userID, id, to)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	err := s.repo.SoftDelete(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return notFound()
		}
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) Settings(ctx context.Context, userID, id uuid.UUID) (*Settings, error) {
	settings, err := s.repo.GetSettings(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return settings, nil
}

func (s *Service) UpdateSettings(ctx context.Context, userID, id uuid.UUID, in SettingsInput) (*Settings, error) {
	current, err := s.repo.GetSettings(ctx, userID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	next, err := s.buildSettings(*current, in)
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.UpdateSettings(ctx, userID, id, next)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return updated, nil
}

func (s *Service) Dashboard(ctx context.Context, userID, id uuid.UUID) (*Event, error) {
	return s.Get(ctx, userID, id)
}

func (s *Service) buildSettings(current Settings, in SettingsInput) (Settings, error) {
	next := current
	if next.Visibility == "" {
		next.Visibility = VisibilityPublic
	}
	if in.Visibility != nil {
		if !in.Visibility.Valid() {
			return Settings{}, validationError("Invalid visibility")
		}
		next.Visibility = *in.Visibility
	}
	if in.AllowDownload != nil {
		next.AllowDownload = *in.AllowDownload
	}
	if in.AllowOriginalDownload != nil {
		next.AllowOriginalDownload = *in.AllowOriginalDownload
	}
	if in.WatermarkEnabled != nil {
		next.WatermarkEnabled = *in.WatermarkEnabled
	}

	if in.Password != nil {
		if *in.Password != "" {
			hash, err := s.hash(*in.Password)
			if err != nil {
				return Settings{}, apperr.Internal().WithCause(err)
			}
			if hash != next.PasswordHash {
				now := s.now().UTC().Truncate(time.Second)
				next.PasswordChangedAt = &now
			}
			next.PasswordHash = hash
			if in.Visibility == nil && next.Visibility == VisibilityPublic {
				next.Visibility = VisibilityPassword
			}
		} else {
			if next.PasswordHash != "" {
				now := s.now().UTC().Truncate(time.Second)
				next.PasswordChangedAt = &now
			}
			next.PasswordHash = ""
		}
	}

	if next.Visibility == VisibilityPassword && next.PasswordHash == "" {
		return Settings{}, validationError("A password is required when visibility is password")
	}
	if next.Visibility == VisibilityPrivate {
		next.AllowDownload = false
		next.AllowOriginalDownload = false
	}
	if !next.AllowDownload {
		next.AllowOriginalDownload = false
	}
	return next, nil
}

func (s *Service) uniqueSlug(ctx context.Context, userID uuid.UUID, name string) (string, error) {
	base := Slugify(name)
	if base == "" {
		base = "event"
	}
	slug := base
	for i := 2; i < 1000; i++ {
		exists, err := s.repo.SlugExists(ctx, userID, slug)
		if err != nil {
			return "", apperr.Internal().WithCause(err)
		}
		if !exists {
			return slug, nil
		}
		slug = SuffixSlug(base, i)
	}
	return "", apperr.Internal().WithCause(errors.New("could not generate unique slug"))
}

func (s *Service) enforceActiveLimit(ctx context.Context, userID uuid.UUID) error {
	max, err := s.limits.MaxActiveEvents(ctx, userID.String())
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	if max <= 0 {
		return nil
	}
	count, err := s.repo.CountByStatus(ctx, userID, StatusActive)
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	if count >= max {
		return apperr.New("PLAN_LIMIT_REACHED", "activeEvents limit reached for your plan", 402)
	}
	return nil
}

func validEmail(email string) bool {
	if len(email) > 320 {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
