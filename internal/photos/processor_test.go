package photos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/imaging"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	gets    []string
	puts    []string
	getErr  error
	putErr  error
}

func newFakeStore() *fakeStore {
	return &fakeStore{objects: map[string][]byte{}}
}

func (f *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets = append(f.gets, key)
	if f.getErr != nil {
		return nil, f.getErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, errors.New("missing object")
	}
	return data, nil
}

func (f *fakeStore) Put(_ context.Context, key, _ string, body []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.putErr != nil {
		return f.putErr
	}
	f.objects[key] = body
	f.puts = append(f.puts, key)
	return nil
}

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	return nil
}

type fakeRepo struct {
	mu     sync.Mutex
	photos map[uuid.UUID]*Photo
	ready  int
	getErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{photos: map[uuid.UUID]*Photo{}}
}

func (f *fakeRepo) Create(context.Context, CreateInput) (*Photo, error) { return nil, nil }
func (f *fakeRepo) GetByID(_ context.Context, id uuid.UUID) (*Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	p, ok := f.photos[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}
func (f *fakeRepo) GetByIDForEvent(context.Context, uuid.UUID, uuid.UUID) (*Photo, error) {
	return nil, nil
}
func (f *fakeRepo) MarkProcessing(context.Context, uuid.UUID, int64) (*Photo, bool, error) {
	return nil, false, nil
}
func (f *fakeRepo) MarkReady(_ context.Context, id uuid.UUID, width, height int, d Derivatives) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.photos[id]
	if !ok {
		return ErrNotFound
	}
	p.Status = StatusReady
	p.Width = &width
	p.Height = &height
	p.ThumbnailKey = &d.Thumbnail.Key
	p.MediumKey = &d.Medium.Key
	p.OptimizedKey = &d.Optimized.Key
	f.ready++
	return nil
}
func (f *fakeRepo) MarkFailed(context.Context, uuid.UUID, string) error        { return nil }
func (f *fakeRepo) SavePartETag(context.Context, uuid.UUID, int, string) error { return nil }
func (f *fakeRepo) EventOwner(_ context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.photos {
		if p.EventID == eventID {
			return uuid.New(), nil
		}
	}
	return uuid.New(), nil
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func newProcessor(t *testing.T) (*Processor, *fakeRepo, *fakeStore) {
	t.Helper()
	repo := newFakeRepo()
	store := newFakeStore()
	std := imaging.New()
	return NewProcessor(repo, store, std, std, std, slog.Default(), repo.EventOwner), repo, store
}

func TestProcessor_GeneratesDerivativesAndMarksReady(t *testing.T) {
	proc, repo, store := newProcessor(t)

	photoID := uuid.New()
	eventID := uuid.New()
	key := "tenant/u/events/e/originals/p/a.jpg"
	photo := &Photo{ID: photoID, EventID: eventID, StorageKey: key, Status: StatusProcessing}
	repo.photos[photoID] = photo
	store.objects[key] = jpegBytes(t, 800, 600)

	payload, err := json.Marshal(ProcessPayload{PhotoID: photoID, EventID: eventID})
	require.NoError(t, err)
	require.NoError(t, proc.Handle(context.Background(), payload))

	require.Equal(t, 1, repo.ready)
	require.Equal(t, StatusReady, repo.photos[photoID].Status)
	require.Equal(t, 800, *repo.photos[photoID].Width)
	require.Equal(t, 600, *repo.photos[photoID].Height)
	require.Len(t, store.puts, 3)
	require.Contains(t, store.puts[0], "thumbnails/"+photoID.String()+".webp")
	require.Contains(t, store.puts[1], "medium/"+photoID.String()+".webp")
	require.Contains(t, store.puts[2], "optimized/"+photoID.String()+".webp")
}

func TestProcessor_UsesPerDerivativeQuality(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	std := imaging.New()
	spy := &spyEncoder{inner: std}
	proc := NewProcessor(repo, store, std, spy, std, slog.Default(), repo.EventOwner)

	photoID := uuid.New()
	key := "tenant/u/events/e/originals/p/a.jpg"
	repo.photos[photoID] = &Photo{ID: photoID, EventID: uuid.New(), StorageKey: key, Status: StatusProcessing}
	store.objects[key] = jpegBytes(t, 800, 600)

	payload, _ := json.Marshal(ProcessPayload{PhotoID: photoID})
	require.NoError(t, proc.Handle(context.Background(), payload))

	require.Equal(t, []int{ThumbnailQuality, MediumQuality, OptimizedQuality}, spy.qualities)
}

type spyEncoder struct {
	inner     imaging.Encoder
	qualities []int
}

func (s *spyEncoder) Encode(img image.Image, quality int) ([]byte, error) {
	s.qualities = append(s.qualities, quality)
	return s.inner.Encode(img, quality)
}

func TestProcessor_IdempotentWhenAlreadyReady(t *testing.T) {
	proc, repo, store := newProcessor(t)
	photoID := uuid.New()
	repo.photos[photoID] = &Photo{ID: photoID, EventID: uuid.New(), StorageKey: "k", Status: StatusReady}

	payload, _ := json.Marshal(ProcessPayload{PhotoID: photoID})
	require.NoError(t, proc.Handle(context.Background(), payload))
	require.Empty(t, store.gets, "should not download an already-ready photo")
	require.Zero(t, repo.ready)
}

func TestProcessor_MissingPhotoIsNoOp(t *testing.T) {
	proc, _, store := newProcessor(t)
	payload, _ := json.Marshal(ProcessPayload{PhotoID: uuid.New()})
	require.NoError(t, proc.Handle(context.Background(), payload))
	require.Empty(t, store.gets)
}

func TestProcessor_RejectsCorruptImage(t *testing.T) {
	proc, repo, store := newProcessor(t)
	photoID := uuid.New()
	key := "k"
	repo.photos[photoID] = &Photo{ID: photoID, EventID: uuid.New(), StorageKey: key, Status: StatusProcessing}
	store.objects[key] = []byte("definitely not an image")

	payload, _ := json.Marshal(ProcessPayload{PhotoID: photoID})
	err := proc.Handle(context.Background(), payload)
	require.Error(t, err)
	require.Zero(t, repo.ready)
	require.Empty(t, store.puts)
}

func TestProcessor_FailsWhenDownloadErrors(t *testing.T) {
	proc, repo, store := newProcessor(t)
	photoID := uuid.New()
	repo.photos[photoID] = &Photo{ID: photoID, EventID: uuid.New(), StorageKey: "k", Status: StatusProcessing}
	store.getErr = errors.New("network down")

	payload, _ := json.Marshal(ProcessPayload{PhotoID: photoID})
	err := proc.Handle(context.Background(), payload)
	require.Error(t, err)
	require.Zero(t, repo.ready)
}

func TestProcessor_FailedPhotoReturnsError(t *testing.T) {
	proc, repo, _ := newProcessor(t)
	photoID := uuid.New()
	repo.photos[photoID] = &Photo{ID: photoID, EventID: uuid.New(), StorageKey: "k", Status: StatusFailed}

	payload, _ := json.Marshal(ProcessPayload{PhotoID: photoID})
	err := proc.Handle(context.Background(), payload)
	require.Error(t, err)
}

func TestProcessor_InvalidPayload(t *testing.T) {
	proc, _, _ := newProcessor(t)
	require.Error(t, proc.Handle(context.Background(), []byte("{")))
	require.Error(t, proc.Handle(context.Background(), []byte("{}")), "missing photoId")
}

func TestCleanupHandler_DeletesOriginal(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	photoID := uuid.New()
	repo.photos[photoID] = &Photo{ID: photoID, StorageKey: "some/key.jpg", Status: StatusFailed}

	store.objects["some/key.jpg"] = []byte("x")
	h := CleanupHandler(repo, store, slog.Default())
	payload, _ := json.Marshal(CleanupPayload{PhotoID: photoID})
	require.NoError(t, h(context.Background(), payload))
	require.Empty(t, store.objects)
}

func TestCleanupHandler_MissingPhotoIsNoOp(t *testing.T) {
	h := CleanupHandler(newFakeRepo(), newFakeStore(), slog.Default())
	payload, _ := json.Marshal(CleanupPayload{PhotoID: uuid.New()})
	require.NoError(t, h(context.Background(), payload))
}

func TestCleanupHandler_InvalidPayload(t *testing.T) {
	h := CleanupHandler(newFakeRepo(), newFakeStore(), slog.Default())
	require.Error(t, h(context.Background(), []byte("{")))
	require.Error(t, h(context.Background(), []byte("{}")))
}

func TestCleanupHandler_GetByIDError(t *testing.T) {
	repo := newFakeRepo()
	repo.getErr = errors.New("db down")
	h := CleanupHandler(repo, newFakeStore(), slog.Default())
	payload, _ := json.Marshal(CleanupPayload{PhotoID: uuid.New()})
	require.Error(t, h(context.Background(), payload))
}
