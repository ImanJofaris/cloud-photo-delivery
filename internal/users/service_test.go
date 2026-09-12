package users

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type fakeRepo struct {
	users map[uuid.UUID]*User
}

func newFakeRepo() *fakeRepo { return &fakeRepo{users: map[uuid.UUID]*User{}} }

func (f *fakeRepo) Create(ctx context.Context, email, passwordHash, businessName string) (*User, error) {
	u := &User{ID: uuid.New(), Email: email, PasswordHash: passwordHash, BusinessName: businessName, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	f.users[u.ID] = u
	return u, nil
}
func (f *fakeRepo) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}
func (f *fakeRepo) GetByEmail(ctx context.Context, email string) (*User, error) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeRepo) UpdateBusinessName(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	u.BusinessName = name
	return u, nil
}
func (f *fakeRepo) RecordFailedLogin(ctx context.Context, id uuid.UUID, maxFailed int, lockUntil time.Time) error {
	return nil
}
func (f *fakeRepo) ResetFailedLogin(ctx context.Context, id uuid.UUID) error { return nil }
func (f *fakeRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	return nil
}

func TestGetProfile_NotFound(t *testing.T) {
	svc := NewService(newFakeRepo())
	_, err := svc.GetProfile(context.Background(), uuid.New())
	ae, ok := err.(*apperr.Error)
	if !ok || ae.Code != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}

func TestUpdateBusinessName(t *testing.T) {
	repo := newFakeRepo()
	u, _ := repo.Create(context.Background(), "a@b.com", "hash", "Old")
	svc := NewService(repo)

	updated, err := svc.UpdateBusinessName(context.Background(), u.ID, "New Booth")
	if err != nil {
		t.Fatal(err)
	}
	if updated.BusinessName != "New Booth" {
		t.Fatalf("got %q", updated.BusinessName)
	}
}

func TestUpdateBusinessName_TooLong(t *testing.T) {
	repo := newFakeRepo()
	u, _ := repo.Create(context.Background(), "a@b.com", "hash", "Old")
	svc := NewService(repo)

	long := make([]byte, 256)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.UpdateBusinessName(context.Background(), u.ID, string(long))
	ae, ok := err.(*apperr.Error)
	if !ok || ae.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %v", err)
	}
}
