package uploads

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

type fakePhotoRepo struct {
	mu             sync.Mutex
	byID           map[uuid.UUID]*photos.Photo
	parts          map[uuid.UUID]map[int]string
	markProcessing int
	createCalls    int
}

func newFakePhotoRepo() *fakePhotoRepo {
	return &fakePhotoRepo{
		byID:  map[uuid.UUID]*photos.Photo{},
		parts: map[uuid.UUID]map[int]string{},
	}
}

func (f *fakePhotoRepo) Create(_ context.Context, in photos.CreateInput) (*photos.Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	p := &photos.Photo{
		ID:                uuid.New(),
		EventID:           in.EventID,
		StorageKey:        in.StorageKey,
		OriginalFilename:  &in.OriginalFilename,
		MimeType:          in.MimeType,
		FileSize:          in.FileSize,
		Status:            in.Status,
		UploadKind:        in.UploadKind,
		MultipartUploadID: in.MultipartUploadID,
	}
	f.byID[p.ID] = p
	return p, nil
}

func (f *fakePhotoRepo) GetByID(_ context.Context, id uuid.UUID) (*photos.Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return nil, photos.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *fakePhotoRepo) GetByIDForEvent(_ context.Context, eventID, id uuid.UUID) (*photos.Photo, error) {
	p, err := f.GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}
	if p.EventID != eventID {
		return nil, photos.ErrNotFound
	}
	return p, nil
}

func (f *fakePhotoRepo) MarkProcessing(_ context.Context, id uuid.UUID, _ int64) (*photos.Photo, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return nil, false, photos.ErrNotFound
	}
	if p.Status != photos.StatusUploading {
		cp := *p
		return &cp, false, nil
	}
	p.Status = photos.StatusProcessing
	f.markProcessing++
	cp := *p
	return &cp, true, nil
}

func (f *fakePhotoRepo) MarkReady(_ context.Context, id uuid.UUID, width, height int, d photos.Derivatives) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return photos.ErrNotFound
	}
	p.Status = photos.StatusReady
	p.Width = &width
	p.Height = &height
	p.ThumbnailKey = &d.Thumbnail.Key
	p.MediumKey = &d.Medium.Key
	p.OptimizedKey = &d.Optimized.Key
	return nil
}

func (f *fakePhotoRepo) MarkFailed(_ context.Context, id uuid.UUID, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.byID[id]
	if !ok {
		return photos.ErrNotFound
	}
	p.Status = photos.StatusFailed
	p.ErrorMessage = &msg
	return nil
}

func (f *fakePhotoRepo) SavePartETag(_ context.Context, photoID uuid.UUID, partNumber int, etag string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.parts[photoID] == nil {
		f.parts[photoID] = map[int]string{}
	}
	f.parts[photoID][partNumber] = etag
	return nil
}

func (f *fakePhotoRepo) EventOwner(_ context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.byID {
		if p.EventID == eventID {
			return p.EventID, nil
		}
	}
	return uuid.Nil, photos.ErrNotFound
}

type fakeUploadRepo struct {
	owned       map[uuid.UUID]bool
	owners      map[uuid.UUID]uuid.UUID
	idempotency map[string]IdempotencyRecord
	partETags   map[uuid.UUID]map[int]string
}

func newFakeUploadRepo() *fakeUploadRepo {
	return &fakeUploadRepo{
		owned:       map[uuid.UUID]bool{},
		owners:      map[uuid.UUID]uuid.UUID{},
		idempotency: map[string]IdempotencyRecord{},
		partETags:   map[uuid.UUID]map[int]string{},
	}
}

func (f *fakeUploadRepo) EventOwnedBy(_ context.Context, userID, eventID uuid.UUID) (bool, error) {
	owner, ok := f.owners[eventID]
	if !ok {
		return f.owned[eventID], nil
	}
	return owner == userID, nil
}

func (f *fakeUploadRepo) LookupIdempotency(_ context.Context, key string) (*IdempotencyRecord, error) {
	rec, ok := f.idempotency[key]
	if !ok {
		return nil, ErrNotFound
	}
	cp := rec
	return &cp, nil
}

func (f *fakeUploadRepo) SaveIdempotency(_ context.Context, key string, rec IdempotencyRecord) error {
	f.idempotency[key] = rec
	return nil
}

func (f *fakeUploadRepo) PartETags(_ context.Context, photoID uuid.UUID) (map[int]string, error) {
	out := map[int]string{}
	for k, v := range f.partETags[photoID] {
		out[k] = v
	}
	return out, nil
}

type fakeStore struct {
	headSize    int64
	headErr     error
	multipartID string
	completeErr error
	putURL      string
	partURL     string
	presigned   []string
	deletes     []string
	aborts      []string
}

func (f *fakeStore) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	if f.putURL != "" {
		return f.putURL, nil
	}
	return "https://example.test/put", nil
}

func (f *fakeStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://example.test/get", nil
}

func (f *fakeStore) Head(context.Context, string) (int64, error) { return f.headSize, f.headErr }

func (f *fakeStore) Delete(_ context.Context, key string) error {
	f.deletes = append(f.deletes, key)
	return nil
}

func (f *fakeStore) Put(context.Context, string, string, []byte) error { return nil }

func (f *fakeStore) Get(context.Context, string) ([]byte, error) { return nil, nil }

func (f *fakeStore) Bucket() string { return "test" }

func (f *fakeStore) CreateMultipartUpload(context.Context, string, string) (string, error) {
	if f.multipartID != "" {
		return f.multipartID, nil
	}
	return "multipart-id", nil
}

func (f *fakeStore) PresignUploadPart(_ context.Context, _ string, _ string, partNumber int, _ time.Duration) (string, error) {
	f.presigned = append(f.presigned, "part")
	if f.partURL != "" {
		return f.partURL, nil
	}
	return "https://example.test/part", nil
}

func (f *fakeStore) CompleteMultipartUpload(context.Context, string, string, []r2.CompletePart) error {
	return f.completeErr
}

func (f *fakeStore) AbortMultipartUpload(_ context.Context, _ string, _ string) error {
	f.aborts = append(f.aborts, "abort")
	return nil
}

type fakeQueue struct {
	enqueued []uuid.UUID
	err      error
}

func (f *fakeQueue) EnqueueProcessPhoto(_ context.Context, photoID, _ uuid.UUID) error {
	if f.err != nil {
		return f.err
	}
	f.enqueued = append(f.enqueued, photoID)
	return nil
}
