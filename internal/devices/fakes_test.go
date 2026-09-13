package devices

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type fakeRepo struct {
	mu        sync.Mutex
	byID      map[uuid.UUID]*Device
	events    map[uuid.UUID]uuid.UUID
	touches   []uuid.UUID
	createErr error
	listErr   error
	getErr    error
	prefixErr error
	renameErr error
	rotateErr error
	revokeErr error
	touchErr  error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		byID:   map[uuid.UUID]*Device{},
		events: map[uuid.UUID]uuid.UUID{},
	}
}

func (f *fakeRepo) Create(_ context.Context, d *Device) (*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	cp := *d
	f.byID[d.ID] = &cp
	return &cp, nil
}

func (f *fakeRepo) ListByUser(_ context.Context, userID uuid.UUID) ([]*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []*Device{}
	for _, d := range f.byID {
		if d.UserID == userID {
			cp := *d
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) GetByID(_ context.Context, userID, id uuid.UUID) (*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	d, ok := f.byID[id]
	if !ok || d.UserID != userID {
		return nil, ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (f *fakeRepo) GetByPrefix(_ context.Context, prefix string) (*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.prefixErr != nil {
		return nil, f.prefixErr
	}
	for _, d := range f.byID {
		if d.KeyPrefix == prefix {
			cp := *d
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Rename(_ context.Context, userID, id uuid.UUID, name string) (*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.renameErr != nil {
		return nil, f.renameErr
	}
	d, ok := f.byID[id]
	if !ok || d.UserID != userID {
		return nil, ErrNotFound
	}
	d.Name = name
	cp := *d
	return &cp, nil
}

func (f *fakeRepo) Rotate(_ context.Context, userID, id uuid.UUID, prefix, hash string) (*Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rotateErr != nil {
		return nil, f.rotateErr
	}
	d, ok := f.byID[id]
	if !ok || d.UserID != userID {
		return nil, ErrNotFound
	}
	d.KeyPrefix = prefix
	d.KeyHash = hash
	cp := *d
	return &cp, nil
}

func (f *fakeRepo) Revoke(_ context.Context, userID, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.revokeErr != nil {
		return f.revokeErr
	}
	d, ok := f.byID[id]
	if !ok || d.UserID != userID {
		return ErrNotFound
	}
	d.RevokedAt = &at
	return nil
}

func (f *fakeRepo) TouchLastUsed(_ context.Context, id uuid.UUID, at time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.touchErr != nil {
		return f.touchErr
	}
	if d, ok := f.byID[id]; ok {
		t := at
		d.LastUsedAt = &t
	}
	f.touches = append(f.touches, id)
	return nil
}

func (f *fakeRepo) EventOwnedBy(_ context.Context, userID, eventID uuid.UUID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	owner, ok := f.events[eventID]
	return ok && owner == userID, nil
}

func (f *fakeRepo) seed(userID uuid.UUID, name string, assigned *uuid.UUID) (*Device, string) {
	raw, prefix, hash, _ := GenerateKey()
	d := &Device{
		ID:              uuid.New(),
		UserID:          userID,
		Name:            name,
		KeyPrefix:       prefix,
		KeyHash:         hash,
		AssignedEventID: assigned,
		CreatedAt:       time.Now().UTC(),
	}
	f.mu.Lock()
	f.byID[d.ID] = d
	f.mu.Unlock()
	return d, raw
}

func (f *fakeRepo) touchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.touches)
}
