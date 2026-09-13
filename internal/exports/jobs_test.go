package exports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
	"github.com/stretchr/testify/require"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func testPhoto(eventID uuid.UUID, name, key, content string, createdAt time.Time) *photos.Photo {
	p := &photos.Photo{
		ID:               uuid.New(),
		EventID:          eventID,
		StorageKey:       key,
		OriginalFilename: &name,
		MimeType:         "image/jpeg",
		Status:           photos.StatusReady,
		CreatedAt:        createdAt,
		UpdatedAt:        createdAt,
	}
	return p
}

func payloadFor(exportID, eventID uuid.UUID) []byte {
	raw, _ := json.Marshal(GeneratePayload{ExportID: exportID, EventID: eventID})
	return raw
}

func readZipEntries(t *testing.T, data []byte) map[string]string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		require.NoError(t, err)
		body, err := io.ReadAll(rc)
		require.NoError(t, err)
		require.NoError(t, rc.Close())
		out[f.Name] = string(body)
	}
	return out
}

func TestGenerateHandler_BuildsZipFromOriginals(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	export := repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	now := time.Now().UTC()
	source := &fakeSource{all: []*photos.Photo{
		testPhoto(eventID, "b.jpg", "originals/b.jpg", "bee", now),
		testPhoto(eventID, "a.jpg", "originals/a.jpg", "ay", now.Add(-time.Second)),
		testPhoto(eventID, "c.jpg", "originals/c.jpg", "sea", now.Add(-2*time.Second)),
	}}
	store.objects["originals/a.jpg"] = []byte("ay")
	store.objects["originals/b.jpg"] = []byte("bee")
	store.objects["originals/c.jpg"] = []byte("sea")

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, 24*time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))

	stored, err := repo.GetByID(ctx, export.ID)
	require.NoError(t, err)
	require.Equal(t, StatusReady, stored.Status)
	require.NotNil(t, stored.ObjectKey)
	require.Equal(t, r2.ExportKey(ownerID, eventID, exportID), *stored.ObjectKey)
	require.NotNil(t, stored.ExpiresAt)

	require.Equal(t, 1, store.putCount())
	put := store.puts[0]
	require.Equal(t, "application/zip", put.ContentType)
	require.Equal(t, int64(len(put.Body)), put.Size)
	require.Equal(t, put.Size, *stored.FileSize)

	entries := readZipEntries(t, put.Body)
	require.Len(t, entries, 3)
	require.Equal(t, "bee", entries["b.jpg"])
	require.Equal(t, "ay", entries["a.jpg"])
	require.Equal(t, "sea", entries["c.jpg"])
}

func TestGenerateHandler_DedupesEntryNames(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	now := time.Now().UTC()
	first := testPhoto(eventID, "IMG.jpg", "k1", "first", now)
	second := testPhoto(eventID, "img.JPG", "k2", "second", now.Add(-time.Second))
	source := &fakeSource{all: []*photos.Photo{second, first}}
	store.objects["k1"] = []byte("first")
	store.objects["k2"] = []byte("second")

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))

	entries := readZipEntries(t, store.puts[0].Body)
	require.Equal(t, "first", entries["IMG.jpg"])
	require.Equal(t, "second", entries["img (2).JPG"])
}

func TestGenerateHandler_FallsBackToGeneratedName(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	photo := testPhoto(eventID, "photo.jpg", "key", "data", time.Now().UTC())
	photo.OriginalFilename = nil
	photo.MimeType = "image/png"
	source := &fakeSource{all: []*photos.Photo{photo}}
	store.objects["key"] = []byte("data")

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))

	entries := readZipEntries(t, store.puts[0].Body)
	require.Contains(t, entries, "photo-"+photo.ID.String()+".png")
}

func TestGenerateHandler_SkipsMissingObject(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	now := time.Now().UTC()
	source := &fakeSource{all: []*photos.Photo{
		testPhoto(eventID, "gone.jpg", "missing", "", now),
		testPhoto(eventID, "here.jpg", "present", "ok", now.Add(-time.Second)),
	}}
	store.objects["present"] = []byte("ok")

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))

	stored, err := repo.GetByID(ctx, exportID)
	require.NoError(t, err)
	require.Equal(t, StatusReady, stored.Status)
	entries := readZipEntries(t, store.puts[0].Body)
	require.Len(t, entries, 1)
	require.Equal(t, "ok", entries["here.jpg"])
}

func TestGenerateHandler_StorageErrorRetries(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	source := &fakeSource{all: []*photos.Photo{testPhoto(eventID, "a.jpg", "boom", "", time.Now().UTC())}}
	store.getErrs["boom"] = errors.New("storage down")

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, time.Hour, discardLogger())
	require.Error(t, handler(ctx, payloadFor(exportID, eventID)))

	stored, err := repo.GetByID(ctx, exportID)
	require.NoError(t, err)
	require.Equal(t, StatusProcessing, stored.Status, "transient failure must leave the export retryable")
	require.Zero(t, store.putCount())
}

func TestGenerateHandler_IdempotentForTerminalExports(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID := uuid.New(), uuid.New()
	key := "ready-key"
	ready := repo.seed(&Export{ID: uuid.New(), EventID: eventID, Status: StatusReady, ObjectKey: &key})
	expired := repo.seed(&Export{ID: uuid.New(), EventID: eventID, Status: StatusExpired})

	handler := GenerateHandler(repo, fixedOwner(ownerID), &fakeSource{}, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(ready.ID, eventID)))
	require.NoError(t, handler(ctx, payloadFor(expired.ID, eventID)))
	require.Zero(t, store.putCount())
}

func TestGenerateHandler_MissingExportIsNoop(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	handler := GenerateHandler(repo, fixedOwner(uuid.New()), &fakeSource{}, store, time.Hour, discardLogger())

	require.NoError(t, handler(context.Background(), payloadFor(uuid.New(), uuid.New())))
	require.Zero(t, store.putCount())
}

func TestGenerateHandler_MissingEventIsNoop(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	eventID, exportID := uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	owner := func(context.Context, uuid.UUID) (uuid.UUID, error) { return uuid.Nil, photos.ErrNotFound }
	handler := GenerateHandler(repo, owner, &fakeSource{}, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))
	require.Zero(t, store.putCount())
}

func TestGenerateHandler_DecodeError(t *testing.T) {
	handler := GenerateHandler(newFakeRepo(), fixedOwner(uuid.New()), &fakeSource{}, newFakeStore(), time.Hour, discardLogger())
	require.Error(t, handler(context.Background(), []byte("{not json")))
}

func TestGenerateHandler_PagesEveryPhoto(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	ownerID, eventID, exportID := uuid.New(), uuid.New(), uuid.New()
	repo.seed(&Export{ID: exportID, EventID: eventID, Status: StatusPending})

	base := time.Now().UTC()
	var all []*photos.Photo
	for i := 0; i < photoPageSize+2; i++ {
		key := "key-" + uuid.NewString()
		all = append(all, testPhoto(eventID, "p.jpg", key, "x", base.Add(-time.Duration(i)*time.Second)))
		store.objects[key] = []byte("x")
	}
	source := &fakeSource{all: all}

	handler := GenerateHandler(repo, fixedOwner(ownerID), source, store, time.Hour, discardLogger())
	require.NoError(t, handler(ctx, payloadFor(exportID, eventID)))

	require.Equal(t, 2, len(source.cursor), "expected two pages")
	require.Nil(t, source.cursor[0])
	require.NotNil(t, source.cursor[1])
	entries := readZipEntries(t, store.puts[0].Body)
	require.Len(t, entries, photoPageSize+2, "all READY photos belong in the archive")
	for _, name := range []string{"p.jpg", "p (2).jpg"} {
		require.Contains(t, entries, name)
	}
}

func TestCleanupHandler_ExpiresAndFailsStale(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	now := time.Now().UTC()

	key := "export-key"
	past := now.Add(-time.Hour)
	ready := repo.seed(&Export{
		ID: uuid.New(), EventID: uuid.New(), Status: StatusReady,
		ObjectKey: &key, ExpiresAt: &past,
	})
	processing := repo.seed(&Export{
		ID: uuid.New(), EventID: uuid.New(), Status: StatusProcessing,
		UpdatedAt: now.Add(-2 * time.Hour),
	})

	handler := CleanupHandler(repo, store, discardLogger())
	require.NoError(t, handler(ctx, nil))

	storedReady, err := repo.GetByID(ctx, ready.ID)
	require.NoError(t, err)
	require.Equal(t, StatusExpired, storedReady.Status)
	require.Equal(t, []string{key}, store.deletes)

	storedProcessing, err := repo.GetByID(ctx, processing.ID)
	require.NoError(t, err)
	require.Equal(t, StatusFailed, storedProcessing.Status)
	require.NotNil(t, storedProcessing.ErrorMessage)
	require.Equal(t, "export timed out", *storedProcessing.ErrorMessage)
}

func TestCleanupHandler_ToleratesMissingObject(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	now := time.Now().UTC()
	key := "already-gone"
	past := now.Add(-time.Hour)
	export := repo.seed(&Export{
		ID: uuid.New(), EventID: uuid.New(), Status: StatusReady,
		ObjectKey: &key, ExpiresAt: &past,
	})
	store.deleteErr[key] = r2.ErrNotFound

	handler := CleanupHandler(repo, store, discardLogger())
	require.NoError(t, handler(ctx, nil))

	stored, err := repo.GetByID(ctx, export.ID)
	require.NoError(t, err)
	require.Equal(t, StatusExpired, stored.Status)
}

func TestCleanupHandler_DeleteErrorPropagates(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	store := newFakeStore()
	now := time.Now().UTC()
	key := "boom"
	past := now.Add(-time.Hour)
	repo.seed(&Export{
		ID: uuid.New(), EventID: uuid.New(), Status: StatusReady,
		ObjectKey: &key, ExpiresAt: &past,
	})
	store.deleteErr[key] = errors.New("storage down")

	handler := CleanupHandler(repo, store, discardLogger())
	require.Error(t, handler(ctx, nil))
}

func TestUniqueEntryName(t *testing.T) {
	seen := map[string]int{}
	require.Equal(t, "a.jpg", uniqueEntryName(seen, "a.jpg"))
	require.Equal(t, "a (2).jpg", uniqueEntryName(seen, "a.jpg"))
	require.Equal(t, "A (3).JPG", uniqueEntryName(seen, "A.JPG"))
	require.Equal(t, "noext", uniqueEntryName(seen, "noext"))
	require.Equal(t, "noext (2)", uniqueEntryName(seen, "noext"))
}

func fixedOwner(id uuid.UUID) EventOwner {
	return func(context.Context, uuid.UUID) (uuid.UUID, error) { return id, nil }
}
