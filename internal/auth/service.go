package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
)

const minPasswordLength = 8

type Config struct {
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
	PasswordResetTTL time.Duration
	LockoutMaxFailed int
	LockoutDuration  time.Duration
	PublicBaseURL    string
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

type Service struct {
	users  users.Repository
	repo   Repository
	tokens *TokenService
	mailer Mailer
	cfg    Config
	clock  Clock
}

func NewService(userRepo users.Repository, repo Repository, tokens *TokenService, mailer Mailer, cfg Config) *Service {
	return &Service{
		users:  userRepo,
		repo:   repo,
		tokens: tokens,
		mailer: mailer,
		cfg:    cfg,
		clock:  realClock{},
	}
}

func (s *Service) SetClock(c Clock) { s.clock = c }

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	User         *users.User
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) bool {
	if len(email) > 320 {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validatePassword(pw string) *apperr.Error {
	if len(pw) < minPasswordLength {
		return apperr.New("WEAK_PASSWORD", "Password must be at least 8 characters", 422)
	}
	if len(pw) > 128 {
		return apperr.New("WEAK_PASSWORD", "Password must be 128 characters or fewer", 422)
	}
	return nil
}

func (s *Service) Signup(ctx context.Context, email, password, businessName string) (*AuthResult, error) {
	email = normalizeEmail(email)
	if !validateEmail(email) {
		return nil, apperr.New("VALIDATION_ERROR", "A valid email is required", 422)
	}
	if err := validatePassword(password); err != nil {
		return nil, err
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	u, err := s.users.Create(ctx, email, hash, businessName)
	if err != nil {
		if errors.Is(err, users.ErrEmailTaken) {
			return nil, apperr.New("EMAIL_TAKEN", "An account with this email already exists", 409)
		}
		return nil, apperr.Internal().WithCause(err)
	}

	return s.issueTokens(ctx, u)
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	email = normalizeEmail(email)
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, invalidCredentials()
		}
		return nil, apperr.Internal().WithCause(err)
	}

	now := s.clock.Now()
	if u.IsLocked(now) {
		return nil, apperr.New("ACCOUNT_LOCKED", "Account is temporarily locked due to failed login attempts", 423)
	}

	if !VerifyPassword(u.PasswordHash, password) {
		lockUntil := now.Add(s.cfg.LockoutDuration)
		if err := s.users.RecordFailedLogin(ctx, u.ID, s.cfg.LockoutMaxFailed, lockUntil); err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		return nil, invalidCredentials()
	}

	if err := s.users.ResetFailedLogin(ctx, u.ID); err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	return s.issueTokens(ctx, u)
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (*AuthResult, error) {
	hash := HashToken(rawToken)
	rt, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrRefreshNotFound) {
			return nil, apperr.New("INVALID_REFRESH_TOKEN", "Refresh token is invalid", 401)
		}
		return nil, apperr.Internal().WithCause(err)
	}

	now := s.clock.Now()
	if rt.RevokedAt != nil {
		// Reuse of a rotated token: revoke the entire family.
		if err := s.repo.RevokeFamily(ctx, rt.FamilyID); err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		return nil, apperr.New("REFRESH_REUSE", "Refresh token has already been used", 401)
	}
	if now.After(rt.ExpiresAt) {
		return nil, apperr.New("INVALID_REFRESH_TOKEN", "Refresh token has expired", 401)
	}

	u, err := s.users.GetByID(ctx, rt.UserID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	if err := s.repo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	return s.issueTokensInFamily(ctx, u, rt.FamilyID)
}

func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	hash := HashToken(rawToken)
	rt, err := s.repo.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrRefreshNotFound) {
			return nil
		}
		return apperr.Internal().WithCause(err)
	}
	if err := s.repo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Do not reveal whether the email exists.
		return nil
	}

	raw, hash, err := NewOpaqueToken()
	if err != nil {
		return apperr.Internal().WithCause(err)
	}
	expiresAt := s.clock.Now().Add(s.cfg.PasswordResetTTL)
	if err := s.repo.CreatePasswordReset(ctx, u.ID, hash, expiresAt); err != nil {
		return apperr.Internal().WithCause(err)
	}

	resetURL := s.cfg.PublicBaseURL + "/reset-password?token=" + raw
	if err := s.mailer.SendPasswordReset(ctx, u.Email, resetURL); err != nil {
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, rawToken, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}

	hash := HashToken(rawToken)
	pr, err := s.repo.GetPasswordResetByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrResetNotFound) {
			return apperr.New("INVALID_RESET_TOKEN", "Password reset token is invalid", 400)
		}
		return apperr.Internal().WithCause(err)
	}

	now := s.clock.Now()
	if pr.UsedAt != nil || now.After(pr.ExpiresAt) {
		return apperr.New("INVALID_RESET_TOKEN", "Password reset token is invalid or has expired", 400)
	}

	newHash, err := HashPassword(newPassword)
	if err != nil {
		return apperr.Internal().WithCause(err)
	}

	if err := s.repo.MarkPasswordResetUsed(ctx, pr.ID); err != nil {
		return apperr.Internal().WithCause(err)
	}
	if err := s.users.UpdatePassword(ctx, pr.UserID, newHash); err != nil {
		return apperr.Internal().WithCause(err)
	}
	// Invalidate all existing sessions.
	if err := s.repo.RevokeAllForUser(ctx, pr.UserID); err != nil {
		return apperr.Internal().WithCause(err)
	}
	return nil
}

func (s *Service) issueTokens(ctx context.Context, u *users.User) (*AuthResult, error) {
	return s.issueTokensInFamily(ctx, u, uuid.New())
}

func (s *Service) issueTokensInFamily(ctx context.Context, u *users.User, familyID uuid.UUID) (*AuthResult, error) {
	access, err := s.tokens.IssueAccessToken(u.ID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	raw, hash, err := NewOpaqueToken()
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	expiresAt := s.clock.Now().Add(s.cfg.RefreshTTL)
	if err := s.repo.CreateRefreshToken(ctx, u.ID, familyID, hash, expiresAt); err != nil {
		return nil, apperr.Internal().WithCause(err)
	}

	return &AuthResult{
		AccessToken:  access,
		RefreshToken: raw,
		ExpiresIn:    int(s.cfg.AccessTTL.Seconds()),
		User:         u,
	}, nil
}

func invalidCredentials() *apperr.Error {
	return apperr.New("INVALID_CREDENTIALS", "Invalid email or password", 401)
}
