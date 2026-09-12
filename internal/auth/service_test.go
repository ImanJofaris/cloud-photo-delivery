package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
)

// ---- fakes ----

type fakeUsersRepo struct {
	byID    map[uuid.UUID]*users.User
	byEmail map[string]*users.User
	create  func(email, hash, name string) (*users.User, error)
}

func newFakeUsersRepo() *fakeUsersRepo {
	return &fakeUsersRepo{byID: map[uuid.UUID]*users.User{}, byEmail: map[string]*users.User{}}
}

func (f *fakeUsersRepo) Create(ctx context.Context, email, passwordHash, businessName string) (*users.User, error) {
	if f.create != nil {
		return f.create(email, passwordHash, businessName)
	}
	if _, ok := f.byEmail[email]; ok {
		return nil, users.ErrEmailTaken
	}
	u := &users.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: passwordHash,
		BusinessName: businessName,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	f.byID[u.ID] = u
	f.byEmail[email] = u
	return u, nil
}

func (f *fakeUsersRepo) GetByID(ctx context.Context, id uuid.UUID) (*users.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, users.ErrNotFound
}

func (f *fakeUsersRepo) GetByEmail(ctx context.Context, email string) (*users.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, users.ErrNotFound
}

func (f *fakeUsersRepo) UpdateBusinessName(ctx context.Context, id uuid.UUID, name string) (*users.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	u.BusinessName = name
	return u, nil
}

func (f *fakeUsersRepo) RecordFailedLogin(ctx context.Context, id uuid.UUID, maxFailed int, lockUntil time.Time) error {
	u, ok := f.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	u.FailedLoginCount++
	if u.FailedLoginCount >= maxFailed {
		lu := lockUntil
		u.LockedUntil = &lu
	}
	return nil
}

func (f *fakeUsersRepo) ResetFailedLogin(ctx context.Context, id uuid.UUID) error {
	u, ok := f.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	u.FailedLoginCount = 0
	u.LockedUntil = nil
	return nil
}

func (f *fakeUsersRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	u, ok := f.byID[id]
	if !ok {
		return users.ErrNotFound
	}
	u.PasswordHash = passwordHash
	return nil
}

type fakeAuthRepo struct {
	refresh  map[string]*RefreshToken
	resets   map[string]*PasswordReset
	families map[uuid.UUID][]*RefreshToken
}

func newFakeAuthRepo() *fakeAuthRepo {
	return &fakeAuthRepo{
		refresh:  map[string]*RefreshToken{},
		resets:   map[string]*PasswordReset{},
		families: map[uuid.UUID][]*RefreshToken{},
	}
}

func (f *fakeAuthRepo) CreateRefreshToken(ctx context.Context, userID, familyID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	rt := &RefreshToken{ID: uuid.New(), UserID: userID, FamilyID: familyID, ExpiresAt: expiresAt}
	f.refresh[tokenHash] = rt
	f.families[familyID] = append(f.families[familyID], rt)
	return nil
}

func (f *fakeAuthRepo) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	if rt, ok := f.refresh[tokenHash]; ok {
		return rt, nil
	}
	return nil, ErrRefreshNotFound
}

func (f *fakeAuthRepo) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	for _, rt := range f.refresh {
		if rt.ID == id {
			now := time.Now()
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeAuthRepo) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.families[familyID] {
		rt.RevokedAt = &now
	}
	return nil
}

func (f *fakeAuthRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	now := time.Now()
	for _, rt := range f.refresh {
		if rt.UserID == userID {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (f *fakeAuthRepo) CreatePasswordReset(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	f.resets[tokenHash] = &PasswordReset{ID: uuid.New(), UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (f *fakeAuthRepo) GetPasswordResetByHash(ctx context.Context, tokenHash string) (*PasswordReset, error) {
	if pr, ok := f.resets[tokenHash]; ok {
		return pr, nil
	}
	return nil, ErrResetNotFound
}

func (f *fakeAuthRepo) MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error {
	for _, pr := range f.resets {
		if pr.ID == id {
			now := time.Now()
			pr.UsedAt = &now
		}
	}
	return nil
}

type fakeMailer struct {
	sent []string
	err  error
}

func (m *fakeMailer) SendPasswordReset(ctx context.Context, toEmail, resetURL string) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, resetURL)
	return nil
}

// ---- helpers ----

func newTestService(t *testing.T) (*Service, *fakeUsersRepo, *fakeAuthRepo, *fakeMailer) {
	t.Helper()
	userRepo := newFakeUsersRepo()
	authRepo := newFakeAuthRepo()
	mailer := &fakeMailer{}
	tokens := NewTokenService("test-secret", 15*time.Minute, 30*24*time.Hour)
	svc := NewService(userRepo, authRepo, tokens, mailer, Config{
		AccessTTL:        15 * time.Minute,
		RefreshTTL:       30 * 24 * time.Hour,
		PasswordResetTTL: time.Hour,
		LockoutMaxFailed: 3,
		LockoutDuration:  15 * time.Minute,
		PublicBaseURL:    "http://localhost:3000",
	})
	return svc, userRepo, authRepo, mailer
}

// ---- tests ----

func TestSignup_Success(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	res, err := svc.Signup(context.Background(), "User@Example.com", "password123", "Iman Booth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.User.Email != "user@example.com" {
		t.Errorf("email not normalized: %s", res.User.Email)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Error("expected tokens")
	}
	if res.ExpiresIn != 900 {
		t.Errorf("expected 900 expiresIn, got %d", res.ExpiresIn)
	}
}

func TestSignup_WeakPassword(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.Signup(context.Background(), "a@b.com", "short", "")
	if !isCode(err, "WEAK_PASSWORD") {
		t.Fatalf("expected WEAK_PASSWORD, got %v", err)
	}
}

func TestSignup_InvalidEmail(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.Signup(context.Background(), "not-an-email", "password123", "")
	if !isCode(err, "VALIDATION_ERROR") {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	if _, err := svc.Signup(context.Background(), "a@b.com", "password123", ""); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Signup(context.Background(), "a@b.com", "password123", "")
	if !isCode(err, "EMAIL_TAKEN") {
		t.Fatalf("expected EMAIL_TAKEN, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	if _, err := svc.Signup(context.Background(), "a@b.com", "password123", ""); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Login(context.Background(), "a@b.com", "password123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("expected access token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	if _, err := svc.Signup(context.Background(), "a@b.com", "password123", ""); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Login(context.Background(), "a@b.com", "wrongpassword")
	if !isCode(err, "INVALID_CREDENTIALS") {
		t.Fatalf("expected INVALID_CREDENTIALS, got %v", err)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.Login(context.Background(), "nobody@b.com", "password123")
	if !isCode(err, "INVALID_CREDENTIALS") {
		t.Fatalf("expected INVALID_CREDENTIALS, got %v", err)
	}
}

func TestLogin_LockoutAfterMaxFailures(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	if _, err := svc.Signup(context.Background(), "a@b.com", "password123", ""); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, _ = svc.Login(context.Background(), "a@b.com", "wrong")
	}
	_, err := svc.Login(context.Background(), "a@b.com", "password123")
	if !isCode(err, "ACCOUNT_LOCKED") {
		t.Fatalf("expected ACCOUNT_LOCKED, got %v", err)
	}
}

func TestRefresh_RotatesAndDetectsReuse(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	res, err := svc.Signup(context.Background(), "a@b.com", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	old := res.RefreshToken

	rotated, err := svc.Refresh(context.Background(), old)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if rotated.RefreshToken == old {
		t.Fatal("expected a new refresh token")
	}

	// Reusing the old (rotated) token must revoke the family.
	_, err = svc.Refresh(context.Background(), old)
	if !isCode(err, "REFRESH_REUSE") {
		t.Fatalf("expected REFRESH_REUSE, got %v", err)
	}

	// The newly issued token should now also be revoked (family revoked).
	_, err = svc.Refresh(context.Background(), rotated.RefreshToken)
	if !isCode(err, "REFRESH_REUSE") {
		t.Fatalf("expected family revocation, got %v", err)
	}
}

func TestLogout_RevokesToken(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	res, _ := svc.Signup(context.Background(), "a@b.com", "password123", "")
	if err := svc.Logout(context.Background(), res.RefreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	_, err := svc.Refresh(context.Background(), res.RefreshToken)
	if !isCode(err, "REFRESH_REUSE") && !isCode(err, "INVALID_REFRESH_TOKEN") {
		t.Fatalf("expected refresh to fail after logout, got %v", err)
	}
}

func TestPasswordReset_FlowAndSingleUse(t *testing.T) {
	svc, _, _, mailer := newTestService(t)
	if _, err := svc.Signup(context.Background(), "a@b.com", "password123", ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestPasswordReset(context.Background(), "a@b.com"); err != nil {
		t.Fatal(err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("expected 1 reset email, got %d", len(mailer.sent))
	}

	token := extractToken(mailer.sent[0])
	if err := svc.ConfirmPasswordReset(context.Background(), token, "newpassword123"); err != nil {
		t.Fatalf("confirm failed: %v", err)
	}

	// Old password no longer works; new one does.
	if _, err := svc.Login(context.Background(), "a@b.com", "password123"); !isCode(err, "INVALID_CREDENTIALS") {
		t.Fatalf("expected old password to fail, got %v", err)
	}
	if _, err := svc.Login(context.Background(), "a@b.com", "newpassword123"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}

	// Token is single-use.
	if err := svc.ConfirmPasswordReset(context.Background(), token, "another123"); !isCode(err, "INVALID_RESET_TOKEN") {
		t.Fatalf("expected single-use rejection, got %v", err)
	}
}

func TestRequestPasswordReset_UnknownEmail_NoError(t *testing.T) {
	svc, _, _, mailer := newTestService(t)
	if err := svc.RequestPasswordReset(context.Background(), "nobody@b.com"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(mailer.sent) != 0 {
		t.Fatal("no email should be sent for unknown address")
	}
}

// ---- test helpers ----

func isCode(err error, code string) bool {
	ae, ok := err.(*apperr.Error)
	return ok && ae.Code == code
}

func extractToken(resetURL string) string {
	const marker = "token="
	i := len(resetURL)
	for j := 0; j+len(marker) <= len(resetURL); j++ {
		if resetURL[j:j+len(marker)] == marker {
			i = j + len(marker)
			break
		}
	}
	return resetURL[i:]
}
