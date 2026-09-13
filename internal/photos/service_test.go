package photos

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

type ownerRepoFake struct {
	photos        map[uuid.UUID]*Photo
	owners        map[uuid.UUID]uuid.UUID
	listed        []*Photo
	listErr       error
	lastListInput ListInput
	deleteResult  *DeletedPhoto
	deleteErr     error
}

func newOwnerRepoFake() *ownerRepoFake {
	return &ownerRepoFake{
		photos: map[uuid.UUID]*Photo{},
		owners: map[uuid.UUID]uuid.UUID{},
	}
}

func (f *ownerRepoFake) Create(context.Context, CreateInput) (*Photo, error) { return nil, nil }

func (f *ownerRepoFake) GetByID(_ context.Context, id uuid.UUID) (*Photo, error) {
	p, ok := f.photos[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (f *ownerRepoFake) GetByIDForEvent(context.Context, uuid.UUID, uuid.UUID) (*Photo, error) {
	return nil, ErrNotFound
}

func (f *ownerRepoFake) MarkProcessing(context.Context, uuid.UUID, int64) (*Photo, bool, error) {
	return nil, false, nil
}

func (f *ownerRepoFake) MarkReady(context.Context, uuid.UUID, int, int, Derivatives) error {
	return nil
}

func (f *ownerRepoFake) MarkFailed(context.Context, uuid.UUID, string) error { return nil }

func (f *ownerRepoFake) SavePartETag(context.Context, uuid.UUID, int, string) error { return nil }

func (f *ownerRepoFake) EventOwner(context.Context, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, ErrNotFound
}

func (f *ownerRepoFake) EventOwnedBy(_ context.Context, userID, eventID uuid.UUID) (bool, error) {
	owner, ok := f.owners[eventID]
	return ok && owner == userID, nil
}

func (f *ownerRepoFake) ListByEvent(_ context.Context, in ListInput) ([]*Photo, error) {
	f.lastListInput = in
	return f.listed, f.listErr
}

func (f *ownerRepoFake) DeleteOwned(_ context.Context, photoID, _ uuid.UUID) (*DeletedPhoto, error) {
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	if _, ok := f.photos[photoID]; !ok {
		return nil, ErrNotFound
	}
	return f.deleteResult, nil
}

type aborterFake struct {
	aborted bool
	err     error
}

func (f *aborterFake) AbortMultipartUpload(context.Context, string, string) error {
	f.aborted = true
	return f.err
}

type cleanupQueueFake struct {
	payloads []CleanupPayload
	err      error
}

func (f *cleanupQueueFake) EnqueueObjectCleanup(_ context.Context, p CleanupPayload) error {
	if f.err != nil {
		return f.err
	}
	f.payloads = append(f.payloads, p)
	return nil
}

func newTestService(repo *ownerRepoFake) (*Service, *stubPresigner, *aborterFake, *cleanupQueueFake) {
	sp := &stubPresigner{}
	ab := &aborterFake{}
	q := &cleanupQueueFake{}
	return NewService(repo, NewSignedURLGenerator(sp, 5*time.Minute), ab, q), sp, ab, q
}

func ownerFixture() (*ownerRepoFake, uuid.UUID, uuid.UUID, *Photo) {
	userID := uuid.New()
	eventID := uuid.New()
	photo := &Photo{
		ID:         uuid.New(),
		EventID:    eventID,
		StorageKey: "orig/a.jpg",
		Status:     StatusReady,
		UploadKind: KindSimple,
	}
	t, m, o := "t.webp", "m.webp", "o.webp"
	photo.ThumbnailKey = &t
	photo.MediumKey = &m
	photo.OptimizedKey = &o

	repo := newOwnerRepoFake()
	repo.photos[photo.ID] = photo
	repo.owners[eventID] = userID
	return repo, userID, eventID, photo
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func TestService_List_DeviceActorScopedToAssignedEvent(t *testing.T) {
	repo, _, eventID, _ := ownerFixture()
	svc, _, _, _ := newTestService(repo)
	assigned := eventID
	actor := Actor{UserID: uuid.New(), DeviceID: uuid.New(), AssignedEvent: &assigned}

	_, err := svc.List(context.Background(), actor, eventID, "", "", 10)
	require.NoError(t, err)

	other := uuid.New()
	actor.AssignedEvent = &other
	_, err = svc.List(context.Background(), actor, eventID, "", "", 10)
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_List_ForeignEventNotFound(t *testing.T) {
	repo, _, eventID, _ := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	_, err := svc.List(context.Background(), Actor{UserID: uuid.New()}, eventID, "", "", 10)
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_List_InvalidCursor(t *testing.T) {
	repo, userID, eventID, _ := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	_, err := svc.List(context.Background(), Actor{UserID: userID}, eventID, "!!!", "", 10)
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_List_InvalidStatus(t *testing.T) {
	repo, userID, eventID, _ := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	_, err := svc.List(context.Background(), Actor{UserID: userID}, eventID, "", "DONE", 10)
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_List_NextCursorWhenFullPage(t *testing.T) {
	repo, userID, eventID, photo := ownerFixture()
	photo2 := &Photo{ID: uuid.New(), EventID: eventID, Status: StatusProcessing, CreatedAt: photo.CreatedAt.Add(-time.Second)}
	repo.listed = []*Photo{photo, photo2}
	svc, _, _, _ := newTestService(repo)

	result, err := svc.List(context.Background(), Actor{UserID: userID}, eventID, "", "READY", 2)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	require.NotEmpty(t, result.NextCursor)

	decoded, err := DecodeCursor(result.NextCursor)
	require.NoError(t, err)
	require.Equal(t, photo2.ID, decoded.ID)
	require.Equal(t, Status("READY"), repo.lastListInput.Status)
}

func TestService_URL_InvalidVariant(t *testing.T) {
	repo, userID, _, photo := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	_, err := svc.URL(context.Background(), Actor{UserID: userID}, photo.ID, "huge")
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_URL_ForeignPhotoNotFound(t *testing.T) {
	repo, _, _, photo := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	_, err := svc.URL(context.Background(), Actor{UserID: uuid.New()}, photo.ID, "thumbnail")
	require.Equal(t, "PHOTO_NOT_FOUND", codeOf(t, err))
}

func TestService_URL_MissingVariant(t *testing.T) {
	repo, userID, _, photo := ownerFixture()
	photo.MediumKey = nil
	svc, _, _, _ := newTestService(repo)

	_, err := svc.URL(context.Background(), Actor{UserID: userID}, photo.ID, "medium")
	require.Equal(t, "VARIANT_UNAVAILABLE", codeOf(t, err))
}

func TestService_URL_OriginalAllowedForOwner(t *testing.T) {
	repo, userID, _, photo := ownerFixture()
	svc, sp, _, _ := newTestService(repo)

	result, err := svc.URL(context.Background(), Actor{UserID: userID}, photo.ID, "original")
	require.NoError(t, err)
	require.Equal(t, photo.StorageKey, sp.key)
	require.Equal(t, 300, result.ExpiresIn)
}

func TestService_Delete_EnqueuesCleanupWithKeys(t *testing.T) {
	repo, userID, eventID, photo := ownerFixture()
	repo.deleteResult = &DeletedPhoto{
		EventID: eventID,
		Keys:    []string{photo.StorageKey, *photo.ThumbnailKey, *photo.MediumKey, *photo.OptimizedKey},
		Counted: true,
	}
	svc, _, aborter, queue := newTestService(repo)

	require.NoError(t, svc.Delete(context.Background(), Actor{UserID: userID}, photo.ID))
	require.False(t, aborter.aborted, "simple uploads do not abort multipart")
	require.Len(t, queue.payloads, 1)
	require.Equal(t, photo.ID, queue.payloads[0].PhotoID)
	require.Equal(t, eventID, queue.payloads[0].EventID)
	require.Len(t, queue.payloads[0].Keys, 4)
}

func TestService_Delete_MultipartAborts(t *testing.T) {
	repo, userID, eventID, photo := ownerFixture()
	uploadID := "multipart-1"
	photo.UploadKind = KindMultipart
	photo.MultipartUploadID = &uploadID
	photo.Status = StatusUploading
	repo.deleteResult = &DeletedPhoto{EventID: eventID, Keys: []string{photo.StorageKey}}
	svc, _, aborter, queue := newTestService(repo)

	require.NoError(t, svc.Delete(context.Background(), Actor{UserID: userID}, photo.ID))
	require.True(t, aborter.aborted)
	require.Len(t, queue.payloads, 1)
}

func TestService_Delete_ForeignPhotoNotFound(t *testing.T) {
	repo, _, _, photo := ownerFixture()
	svc, _, _, _ := newTestService(repo)

	err := svc.Delete(context.Background(), Actor{UserID: uuid.New()}, photo.ID)
	require.Equal(t, "PHOTO_NOT_FOUND", codeOf(t, err))
}

func TestService_Delete_QueueErrorIsInternal(t *testing.T) {
	repo, userID, eventID, photo := ownerFixture()
	repo.deleteResult = &DeletedPhoto{EventID: eventID, Keys: []string{photo.StorageKey}}
	svc, _, _, queue := newTestService(repo)
	queue.err = errors.New("queue down")

	err := svc.Delete(context.Background(), Actor{UserID: userID}, photo.ID)
	require.Equal(t, "INTERNAL_ERROR", codeOf(t, err))
}

func TestService_Delete_RepoRaceNotFound(t *testing.T) {
	repo, userID, _, photo := ownerFixture()
	repo.deleteErr = ErrNotFound
	svc, _, _, _ := newTestService(repo)

	err := svc.Delete(context.Background(), Actor{UserID: userID}, photo.ID)
	require.Equal(t, "PHOTO_NOT_FOUND", codeOf(t, err))
}

func TestVariantNames(t *testing.T) {
	tk, mk, ok := "t", "m", "o"
	photo := &Photo{ThumbnailKey: &tk, MediumKey: &mk, OptimizedKey: &ok}
	require.Equal(t, []string{"thumbnail", "medium", "large"}, VariantNames(photo))

	photo.ThumbnailKey = nil
	require.Equal(t, []string{"medium", "large"}, VariantNames(photo))

	require.Empty(t, VariantNames(&Photo{}))
}
