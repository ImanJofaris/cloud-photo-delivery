package uploads

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*Service, *fakePhotoRepo, *fakeUploadRepo, *fakeStore, *fakeQueue, uuid.UUID, uuid.UUID) {
	t.Helper()
	photoRepo := newFakePhotoRepo()
	uploadRepo := newFakeUploadRepo()
	store := &fakeStore{headSize: 1024}
	queue := &fakeQueue{}
	svc := NewService(photoRepo, uploadRepo, store, queue)
	svc.SetClock(func() time.Time { return time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC) })

	userID := uuid.New()
	eventID := uuid.New()
	uploadRepo.owned[eventID] = true
	uploadRepo.owners[eventID] = userID
	return svc, photoRepo, uploadRepo, store, queue, userID, eventID
}

func TestInitialize_SimpleUploadForSmallFile(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)

	res, err := svc.Initialize(context.Background(), userID, InitParams{
		EventID: eventID, Filename: "photo.jpg", ContentType: "image/jpeg", Size: 5 * 1024 * 1024,
	})
	require.NoError(t, err)
	require.Equal(t, photos.KindSimple, res.UploadKind)
	require.NotEmpty(t, res.UploadURL)
	require.Equal(t, int64(0), res.PartSize)
	require.Contains(t, res.StorageKey, "/originals/")
}

func TestInitialize_MultipartForLargeFile(t *testing.T) {
	svc, _, _, store, _, userID, eventID := newTestService(t)
	store.multipartID = "up-123"

	res, err := svc.Initialize(context.Background(), userID, InitParams{
		EventID: eventID, Filename: "big.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024,
	})
	require.NoError(t, err)
	require.Equal(t, photos.KindMultipart, res.UploadKind)
	require.Equal(t, int64(PartSize), res.PartSize)
	require.NotEmpty(t, res.UploadURL)
}

func TestInitialize_RejectsUnknownEvent(t *testing.T) {
	svc, _, _, _, _, userID, _ := newTestService(t)

	_, err := svc.Initialize(context.Background(), userID, InitParams{
		EventID: uuid.New(), Filename: "photo.jpg", ContentType: "image/jpeg", Size: 100,
	})
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "EVENT_NOT_FOUND", appErr.Code)
}

func TestInitialize_Validation(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)

	cases := []struct {
		name     string
		filename string
		mime     string
		size     int64
	}{
		{"empty filename", "", "image/jpeg", 100},
		{"unsupported mime", "a.gif", "image/gif", 100},
		{"zero size", "a.jpg", "image/jpeg", 0},
		{"oversized", "a.jpg", "image/jpeg", MaxFileSize + 1},
		{"traversal name is sanitized not rejected", "../x.jpg", "image/jpeg", 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Initialize(context.Background(), userID, InitParams{
				EventID: eventID, Filename: tc.filename, ContentType: tc.mime, Size: tc.size,
			})
			if tc.name == "traversal name is sanitized not rejected" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestInitialize_Idempotency(t *testing.T) {
	svc, photoRepo, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	params := InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 100, IdempotencyKey: "key-1"}
	first, err := svc.Initialize(ctx, userID, params)
	require.NoError(t, err)
	second, err := svc.Initialize(ctx, userID, params)
	require.NoError(t, err)
	require.Equal(t, first.PhotoID, second.PhotoID)
	require.Equal(t, 1, photoRepo.createCalls, "same key + same body must not create a second photo")

	params.Size = 200
	_, err = svc.Initialize(ctx, userID, params)
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "IDEMPOTENCY_CONFLICT", appErr.Code)
}

func TestParts_RequiresMultipart(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 100})
	require.NoError(t, err)

	_, err = svc.Parts(ctx, userID, res.PhotoID, []int{1, 2})
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "NOT_MULTIPART", appErr.Code)
}

func TestParts_ReturnsPresignedURLs(t *testing.T) {
	svc, _, _, store, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024})
	require.NoError(t, err)

	parts, err := svc.Parts(ctx, userID, res.PhotoID, []int{1, 2, 3})
	require.NoError(t, err)
	require.Len(t, parts.Parts, 3)
	require.GreaterOrEqual(t, len(store.presigned), 3)

	_, err = svc.Parts(ctx, userID, res.PhotoID, []int{0})
	require.Error(t, err)
}

func TestComplete_SuccessTransitionsAndEnqueuesOnce(t *testing.T) {
	svc, photoRepo, _, _, queue, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	photo, err := svc.Complete(ctx, userID, res.PhotoID, "")
	require.NoError(t, err)
	require.Equal(t, photos.StatusProcessing, photo.Status)
	require.Len(t, queue.enqueued, 1)
	require.Equal(t, 1, photoRepo.markProcessing)

	again, err := svc.Complete(ctx, userID, res.PhotoID, "")
	require.NoError(t, err)
	require.Equal(t, photos.StatusProcessing, again.Status)
	require.Len(t, queue.enqueued, 1, "second complete must not re-enqueue")
}

func TestComplete_ObjectMissing(t *testing.T) {
	svc, _, _, store, _, userID, eventID := newTestService(t)
	store.headErr = r2.ErrNotFound
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	_, err = svc.Complete(ctx, userID, res.PhotoID, "")
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPLOAD_NOT_FOUND", appErr.Code)
}

func TestComplete_SizeMismatch(t *testing.T) {
	svc, _, _, store, _, userID, eventID := newTestService(t)
	store.headSize = 999
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	_, err = svc.Complete(ctx, userID, res.PhotoID, "")
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "UPLOAD_SIZE_MISMATCH", appErr.Code)
}

func TestComplete_TenantIsolation(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	otherUser := uuid.New()
	_, err = svc.Status(ctx, otherUser, res.PhotoID)
	require.ErrorIs(t, err.(*apperr.Error), notFound())
}

func TestCompleteMultipart_RequiresPartsAndStoresETags(t *testing.T) {
	svc, photoRepo, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024})
	require.NoError(t, err)

	err = svc.CompleteMultipart(ctx, userID, res.PhotoID, nil)
	require.Error(t, err)

	err = svc.CompleteMultipart(ctx, userID, res.PhotoID, []CompletedPart{
		{PartNumber: 2, ETag: "etag-2"},
		{PartNumber: 1, ETag: "etag-1"},
	})
	require.NoError(t, err)
	require.Equal(t, "etag-1", photoRepo.parts[res.PhotoID][1])
	require.Equal(t, "etag-2", photoRepo.parts[res.PhotoID][2])
}

func TestAbortMultipart_MarksFailed(t *testing.T) {
	svc, photoRepo, _, store, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024})
	require.NoError(t, err)

	require.NoError(t, svc.AbortMultipart(ctx, userID, res.PhotoID))
	require.Len(t, store.aborts, 1)
	require.Equal(t, photos.StatusFailed, photoRepo.byID[res.PhotoID].Status)
}

func TestStatus_TenantIsolation(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	got, err := svc.Status(ctx, userID, res.PhotoID)
	require.NoError(t, err)
	require.Equal(t, res.PhotoID, got.ID)
}

func TestComplete_IdempotencyKeyConflict(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	other, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "b.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	_, err = svc.Complete(ctx, userID, res.PhotoID, "complete-key")
	require.NoError(t, err)

	_, err = svc.Complete(ctx, userID, other.PhotoID, "complete-key")
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "IDEMPOTENCY_CONFLICT", appErr.Code)
}

func TestComplete_QueueFailurePropagates(t *testing.T) {
	svc, _, _, _, queue, userID, eventID := newTestService(t)
	queue.err = errors.New("queue down")
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	_, err = svc.Complete(ctx, userID, res.PhotoID, "")
	require.Error(t, err)
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr))
	require.Equal(t, "INTERNAL_ERROR", appErr.Code)
}

func TestParts_UnknownPhoto(t *testing.T) {
	svc, _, _, _, _, userID, _ := newTestService(t)

	_, err := svc.Parts(context.Background(), userID, uuid.New(), []int{1})
	require.ErrorIs(t, err.(*apperr.Error), notFound())
}

func TestMultipart_AllOperationsRequireMultipart(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024})
	require.NoError(t, err)

	_, err = svc.Parts(ctx, userID, res.PhotoID, []int{1})
	require.Equal(t, "NOT_MULTIPART", err.(*apperr.Error).Code)

	err = svc.CompleteMultipart(ctx, userID, res.PhotoID, []CompletedPart{{PartNumber: 1, ETag: "e"}})
	require.Equal(t, "NOT_MULTIPART", err.(*apperr.Error).Code)

	err = svc.AbortMultipart(ctx, userID, res.PhotoID)
	require.Equal(t, "NOT_MULTIPART", err.(*apperr.Error).Code)
}

func TestCompleteMultipart_ValidatesParts(t *testing.T) {
	svc, _, _, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024})
	require.NoError(t, err)

	err = svc.CompleteMultipart(ctx, userID, res.PhotoID, []CompletedPart{{PartNumber: 0, ETag: "e"}})
	require.Equal(t, "VALIDATION_ERROR", err.(*apperr.Error).Code)

	err = svc.CompleteMultipart(ctx, userID, res.PhotoID, []CompletedPart{{PartNumber: 1, ETag: "  "}})
	require.Equal(t, "VALIDATION_ERROR", err.(*apperr.Error).Code)
}

func TestInitialize_IdempotentRebuildNotFound(t *testing.T) {
	svc, photoRepo, uploadRepo, _, _, userID, eventID := newTestService(t)
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024, IdempotencyKey: "k"})
	require.NoError(t, err)
	delete(photoRepo.byID, res.PhotoID)
	uploadRepo.idempotency["k"] = IdempotencyRecord{UserID: userID, PhotoID: res.PhotoID, RequestHash: requestHash(eventID, "a.jpg", "image/jpeg", int64(1024))}

	_, err = svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024, IdempotencyKey: "k"})
	require.ErrorIs(t, err.(*apperr.Error), notFound())
}

func TestNewService_NilQueueIsNoop(t *testing.T) {
	svc := NewService(newFakePhotoRepo(), newFakeUploadRepo(), &fakeStore{}, nil)
	require.NoError(t, svc.queue.EnqueueProcessPhoto(context.Background(), uuid.New(), uuid.New()))
}

type errorStore struct {
	fakeStore
	pushErr bool
}

func (e *errorStore) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	if e.pushErr {
		return "", errors.New("presign failed")
	}
	return "https://example.test/put", nil
}

func (e *errorStore) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "", errors.New("multipart failed")
}

func (e *errorStore) AbortMultipartUpload(context.Context, string, string) error {
	return errors.New("abort failed")
}

func TestInitialize_StoreErrorIsInternal(t *testing.T) {
	photoRepo := newFakePhotoRepo()
	uploadRepo := newFakeUploadRepo()
	userID := uuid.New()
	eventID := uuid.New()
	uploadRepo.owners[eventID] = userID
	svc := NewService(photoRepo, uploadRepo, &errorStore{pushErr: true}, &fakeQueue{})

	_, err := svc.Initialize(context.Background(), userID, InitParams{
		EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 1024,
	})
	require.Equal(t, "INTERNAL_ERROR", err.(*apperr.Error).Code)

	_, err = svc.Initialize(context.Background(), userID, InitParams{
		EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024,
	})
	require.Equal(t, "INTERNAL_ERROR", err.(*apperr.Error).Code)
}

func TestAbortMultipart_StoreErrorIsInternal(t *testing.T) {
	photoRepo := newFakePhotoRepo()
	uploadRepo := newFakeUploadRepo()
	userID := uuid.New()
	eventID := uuid.New()
	uploadRepo.owners[eventID] = userID
	svc := NewService(photoRepo, uploadRepo, &fakeStore{}, &fakeQueue{})
	ctx := context.Background()

	res, err := svc.Initialize(ctx, userID, InitParams{EventID: eventID, Filename: "a.jpg", ContentType: "image/jpeg", Size: 20 * 1024 * 1024})
	require.NoError(t, err)

	svc.store = &errorStore{}
	err = svc.AbortMultipart(ctx, userID, res.PhotoID)
	require.Equal(t, "INTERNAL_ERROR", err.(*apperr.Error).Code)
}
