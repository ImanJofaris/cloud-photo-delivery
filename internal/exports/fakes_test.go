package exports

import (
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

type fakeRepo struct {
	mu         sync.Mutex
	items      map[uuid.UUID]*Export
	owners     map[uuid.UUID]uuid.UUID
	createErr  error
	markReady  []uuid.UUID
	markFailed []failedMark
}

type failedMark struct {
	ID      uuid.UUID
	Message string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		items:  map[uuid.UUID]*Export{},
		owners: map[uuid.UUID]uuid.UUID{},
	}
}

func (f *fakeRepo) seed(e *Export) *Export {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *e
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = cp.CreatedAt
	}
	f.items[cp.ID] = &cp
	return &cp
}

func (f *fakeRepo) Create(_ context.Context, eventID uuid.UUID) (*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return nil, f.createErr
	}
	now := time.Now().UTC()
	e := &Export{ID: uuid.New(), EventID: eventID, Status: StatusPending, CreatedAt: now, UpdatedAt: now}
	f.items[e.ID] = e
	cp := *e
	return &cp, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeRepo) GetOwned(_ context.Context, userID, eventID, exportID uuid.UUID) (*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[exportID]
	if !ok || e.EventID != eventID {
		return nil, ErrNotFound
	}
	if owner, ok := f.owners[eventID]; ok && owner != userID {
		return nil, ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeRepo) GetActiveByEvent(_ context.Context, eventID uuid.UUID) (*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var newest *Export
	for _, e := range f.items {
		if e.EventID != eventID || !e.Status.Active() {
			continue
		}
		if newest == nil || e.CreatedAt.After(newest.CreatedAt) ||
			(e.CreatedAt.Equal(newest.CreatedAt) && e.ID.String() > newest.ID.String()) {
			newest = e
		}
	}
	if newest == nil {
		return nil, ErrNotFound
	}
	cp := *newest
	return &cp, nil
}

func (f *fakeRepo) MarkProcessing(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = StatusProcessing
	e.UpdatedAt = time.Now().UTC()
	return nil
}

func (f *fakeRepo) MarkReady(_ context.Context, id uuid.UUID, objectKey string, size int64, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = StatusReady
	e.ObjectKey = &objectKey
	e.FileSize = &size
	e.ExpiresAt = &expiresAt
	e.UpdatedAt = time.Now().UTC()
	f.markReady = append(f.markReady, id)
	return nil
}

func (f *fakeRepo) MarkFailed(_ context.Context, id uuid.UUID, message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = StatusFailed
	e.ErrorMessage = &message
	e.UpdatedAt = time.Now().UTC()
	f.markFailed = append(f.markFailed, failedMark{ID: id, Message: message})
	return nil
}

func (f *fakeRepo) MarkExpired(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	e, ok := f.items[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = StatusExpired
	e.UpdatedAt = time.Now().UTC()
	return nil
}

func (f *fakeRepo) ListExpired(_ context.Context, now time.Time, limit int) ([]*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*Export
	for _, e := range f.items {
		if e.Status == StatusReady && e.ExpiresAt != nil && !e.ExpiresAt.After(now) {
			cp := *e
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(*out[j].ExpiresAt) })
	return capExports(out, limit), nil
}

func (f *fakeRepo) ListStaleProcessing(_ context.Context, before time.Time, limit int) ([]*Export, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*Export
	for _, e := range f.items {
		if e.Status == StatusProcessing && !e.UpdatedAt.After(before) {
			cp := *e
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.Before(out[j].UpdatedAt) })
	return capExports(out, limit), nil
}

func capExports(items []*Export, limit int) []*Export {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

type fakeEvents struct {
	owned bool
	err   error
}

func (f fakeEvents) EventOwnedBy(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return f.owned, f.err
}

type fakeCounter struct {
	count int64
	err   error
}

func (f fakeCounter) CountByEvent(context.Context, uuid.UUID) (int64, error) {
	return f.count, f.err
}

type fakeQueue struct {
	mu       sync.Mutex
	payloads []GeneratePayload
	err      error
}

func (f *fakeQueue) EnqueueGenerate(_ context.Context, payload GeneratePayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.payloads = append(f.payloads, payload)
	return nil
}

func (f *fakeQueue) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.payloads)
}

type fakePresigner struct {
	mu    sync.Mutex
	ttls  []time.Duration
	keys  []string
	url   string
	err   error
	calls int
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	f.keys = append(f.keys, key)
	f.ttls = append(f.ttls, ttl)
	if f.url != "" {
		return f.url, nil
	}
	return "https://example.test/export.zip", nil
}

// fakeSource serves photos in the same (created_at, id) DESC order as the
// repository and honours the cursor, so handler paging is exercised for real.
type fakeSource struct {
	all    []*photos.Photo
	err    error
	cursor []*photos.Cursor
}

func (f *fakeSource) ListByEvent(_ context.Context, in photos.ListInput) ([]*photos.Photo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.cursor = append(f.cursor, in.Cursor)
	items := make([]*photos.Photo, len(f.all))
	copy(items, f.all)
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})

	var out []*photos.Photo
	for _, p := range items {
		if in.Status != "" && p.Status != in.Status {
			continue
		}
		if in.Cursor != nil && !lessThanCursor(p, in.Cursor) {
			continue
		}
		out = append(out, p)
		if len(out) == in.Limit {
			break
		}
	}
	return out, nil
}

func lessThanCursor(p *photos.Photo, c *photos.Cursor) bool {
	if p.CreatedAt.Equal(c.CreatedAt) {
		return p.ID.String() < c.ID.String()
	}
	return p.CreatedAt.Before(c.CreatedAt)
}

type fakeStore struct {
	mu        sync.Mutex
	objects   map[string][]byte
	getErrs   map[string]error
	putErr    error
	deleteErr map[string]error
	puts      []fakePut
	deletes   []string
}

type fakePut struct {
	Key         string
	ContentType string
	Body        []byte
	Size        int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		objects:   map[string][]byte{},
		getErrs:   map[string]error{},
		deleteErr: map[string]error{},
	}
}

func (f *fakeStore) GetReader(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.getErrs[key]; ok {
		return nil, err
	}
	body, ok := f.objects[key]
	if !ok {
		return nil, r2.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(body)), nil
}

func (f *fakeStore) PutReader(_ context.Context, key, contentType string, r io.Reader, size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.puts = append(f.puts, fakePut{Key: key, ContentType: contentType, Body: body, Size: size})
	return nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deletes = append(f.deletes, key)
	if err, ok := f.deleteErr[key]; ok {
		return err
	}
	return nil
}

func (f *fakeStore) putCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.puts)
}

func (f *fakeStore) deleteCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.deletes)
}
