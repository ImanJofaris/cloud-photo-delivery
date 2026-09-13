package gallery

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
)

type PasswordVerifier func(hash, password string) bool
type Service struct {
	repo     Repository
	urls     *photos.SignedURLGenerator
	tokens   *UnlockTokens
	verify   PasswordVerifier
	branding BrandingProvider
}

func NewService(repo Repository, urls *photos.SignedURLGenerator, tokens *UnlockTokens, verify PasswordVerifier, branding BrandingProvider) *Service {
	return &Service{repo: repo, urls: urls, tokens: tokens, verify: verify, branding: branding}
}

func notFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func unauthorized() *apperr.Error {
	return apperr.New("UNAUTHORIZED", "A valid gallery unlock token is required", 401)
}

func forbidden(code, msg string) *apperr.Error {
	return apperr.New(code, msg, 403)
}

// visibleEvent resolves a slug to an accessible event, applying visibility
// rules. Private, deleted, and expired events are reported as not found so
// existence is never revealed.
func (s *Service) visibleEvent(ctx context.Context, slug, unlockToken string) (*VisibleEvent, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, notFound()
	}
	event, settings, err := s.repo.EventBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if settings.Visibility == events.VisibilityPrivate {
		return nil, notFound()
	}
	if settings.Visibility == events.VisibilityPassword {
		if unlockToken == "" {
			return nil, unauthorized()
		}
		if _, err := s.tokens.Verify(unlockToken, event.ID, settings.PasswordChangedAt); err != nil {
			return nil, unauthorized()
		}
	}
	return &VisibleEvent{Event: event, Settings: settings}, nil
}

func (s *Service) GetEvent(ctx context.Context, slug, unlockToken string) (*VisibleEvent, bool, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, false, notFound()
	}
	event, settings, err := s.repo.EventBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, notFound()
		}
		return nil, false, apperr.Internal().WithCause(err)
	}
	if settings.Visibility == events.VisibilityPrivate {
		return nil, false, notFound()
	}
	requiresUnlock := settings.Visibility == events.VisibilityPassword
	if requiresUnlock && unlockToken != "" {
		if _, err := s.tokens.Verify(unlockToken, event.ID, settings.PasswordChangedAt); err == nil {
			requiresUnlock = false
		}
	}
	branding, err := s.loadBranding(ctx, event.UserID)
	if err != nil {
		return nil, false, err
	}
	return &VisibleEvent{Event: event, Settings: settings, Branding: branding}, requiresUnlock, nil
}

func (s *Service) loadBranding(ctx context.Context, userID uuid.UUID) (*users.BrandingView, error) {
	if s.branding == nil {
		return nil, nil
	}
	view, err := s.branding.Get(ctx, userID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return view, nil
}

func (s *Service) Unlock(ctx context.Context, slug, password string) (*UnlockResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, notFound()
	}
	if password == "" {
		return nil, validationError("Password is required")
	}
	event, settings, err := s.repo.EventBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, notFound()
		}
		return nil, apperr.Internal().WithCause(err)
	}
	if settings.Visibility == events.VisibilityPrivate {
		return nil, notFound()
	}
	if settings.Visibility != events.VisibilityPassword || settings.PasswordHash == "" {
		return nil, validationError("This event is not password protected")
	}
	if !s.verify(settings.PasswordHash, password) {
		return nil, unauthorized()
	}
	token, err := s.tokens.Issue(event.ID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return &UnlockResult{Token: token, ExpiresIn: int(s.tokens.TTL().Seconds())}, nil
}

func (s *Service) ListPhotos(ctx context.Context, slug, unlockToken, cursor string, limit int) (*VisibleEvent, *PhotoPage, error) {
	ve, err := s.visibleEvent(ctx, slug, unlockToken)
	if err != nil {
		return nil, nil, err
	}
	cur, err := DecodeCursor(cursor)
	if err != nil {
		return nil, nil, validationError("Invalid cursor")
	}
	n := NormalizeLimit(limit)
	items, err := s.repo.ListReadyPhotos(ctx, ve.Event.ID, cur, n)
	if err != nil {
		return nil, nil, apperr.Internal().WithCause(err)
	}
	branding, err := s.loadBranding(ctx, ve.Event.UserID)
	if err != nil {
		return nil, nil, err
	}
	ve.Branding = branding
	page := &PhotoPage{Items: items}
	if len(items) == n && len(items) > 0 {
		last := items[len(items)-1]
		page.NextCursor = EncodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return ve, page, nil
}

func (s *Service) GetPhoto(ctx context.Context, slug, unlockToken, photoID string) (*VisibleEvent, *photos.Photo, error) {
	ve, err := s.visibleEvent(ctx, slug, unlockToken)
	if err != nil {
		return nil, nil, err
	}
	id, err := uuid.Parse(photoID)
	if err != nil {
		return nil, nil, notFound()
	}
	photo, err := s.repo.ReadyPhoto(ctx, ve.Event.ID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil, apperr.New("PHOTO_NOT_FOUND", "Photo not found", 404)
		}
		return nil, nil, apperr.Internal().WithCause(err)
	}
	return ve, photo, nil
}

func (s *Service) PhotoURL(ctx context.Context, slug, unlockToken, photoID, variant string) (*photos.URLResult, error) {
	ve, err := s.visibleEvent(ctx, slug, unlockToken)
	if err != nil {
		return nil, err
	}
	v := Variant(strings.ToLower(strings.TrimSpace(variant)))
	if !v.Valid() {
		return nil, validationError("Invalid variant")
	}
	if !ve.Settings.AllowDownload {
		return nil, forbidden("DOWNLOAD_DISABLED", "Downloads are disabled for this event")
	}
	if v.IsOriginal() && !ve.Settings.AllowOriginalDownload {
		return nil, forbidden("ORIGINAL_DOWNLOAD_DISABLED", "Original downloads are disabled for this event")
	}
	id, err := uuid.Parse(photoID)
	if err != nil {
		return nil, apperr.New("PHOTO_NOT_FOUND", "Photo not found", 404)
	}
	photo, err := s.repo.ReadyPhoto(ctx, ve.Event.ID, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, apperr.New("PHOTO_NOT_FOUND", "Photo not found", 404)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	result, err := s.urls.URL(ctx, photo, string(v))
	if err != nil {
		if errors.Is(err, photos.ErrVariantUnavailable) {
			return nil, apperr.New("VARIANT_UNAVAILABLE", "Requested variant is not available", 404)
		}
		return nil, apperr.Internal().WithCause(err)
	}
	return result, nil
}
