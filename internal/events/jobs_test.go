package events

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeNotifier struct {
	warned []ExpiryWarning
	err    error
}

func (f *fakeNotifier) NotifyExpiryWarning(ctx context.Context, w ExpiryWarning) error {
	if f.err != nil {
		return f.err
	}
	f.warned = append(f.warned, w)
	return nil
}

type fakeDeleter struct {
	deleted []string
	failKey string
	err     error
}

func (f *fakeDeleter) Delete(ctx context.Context, key string) error {
	if f.err != nil && key == f.failKey {
		return f.err
	}
	f.deleted = append(f.deleted, key)
	return nil
}

func expiryTestRepo(now time.Time) (*fakeRepo, *Event, *Event, *Event) {
	repo := newFakeRepo()
	past := now.Add(-time.Hour)
	soon := now.Add(3 * 24 * time.Hour)
	later := now.Add(30 * 24 * time.Hour)
	due := &Event{ID: uuid.New(), UserID: uuid.New(), Name: "Due", Status: StatusActive, ExpiresAt: &past}
	warn := &Event{ID: uuid.New(), UserID: uuid.New(), Name: "Soon", Status: StatusActive, ExpiresAt: &soon}
	far := &Event{ID: uuid.New(), UserID: uuid.New(), Name: "Later", Status: StatusUpcoming, ExpiresAt: &later}
	repo.events[due.ID] = due
	repo.events[warn.ID] = warn
	repo.events[far.ID] = far
	return repo, due, warn, far
}

func TestExpireHandler_MarksDueAndWarnsOnce(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo, due, warn, far := expiryTestRepo(now)
	notifier := &fakeNotifier{}
	h := ExpireHandler(repo, notifier, 7*24*time.Hour, func() time.Time { return now }, discardLogger())

	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if due.Status != StatusExpired {
		t.Fatalf("due status = %q, want expired", due.Status)
	}
	if far.Status != StatusUpcoming {
		t.Fatalf("far status = %q", far.Status)
	}
	if len(notifier.warned) != 1 || notifier.warned[0].EventID != warn.ID {
		t.Fatalf("warned = %+v", notifier.warned)
	}
	if notifier.warned[0].OwnerEmail != repo.ownerEmail {
		t.Fatalf("owner email = %q", notifier.warned[0].OwnerEmail)
	}
	if _, ok := repo.warned[warn.ID]; !ok {
		t.Fatal("warning was not persisted")
	}

	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(notifier.warned) != 1 {
		t.Fatalf("second run warned again: %+v", notifier.warned)
	}
}

func TestExpireHandler_WarningFailureDoesNotBlockExpiry(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo, due, warn, _ := expiryTestRepo(now)
	notifier := &fakeNotifier{err: errors.New("mailer down")}
	h := ExpireHandler(repo, notifier, 7*24*time.Hour, func() time.Time { return now }, discardLogger())

	err := h(context.Background(), nil)
	if err == nil {
		t.Fatal("expected warning error to be surfaced")
	}
	if due.Status != StatusExpired {
		t.Fatalf("due status = %q, want expired", due.Status)
	}
	if _, ok := repo.warned[warn.ID]; ok {
		t.Fatal("failed warning must not be marked")
	}
}

func TestExpireHandler_SkipsArchived(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	past := now.Add(-time.Hour)
	archived := &Event{ID: uuid.New(), UserID: uuid.New(), Status: StatusArchived, ExpiresAt: &past}
	repo.events[archived.ID] = archived

	h := ExpireHandler(repo, &fakeNotifier{}, 7*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if archived.Status != StatusArchived {
		t.Fatalf("status = %q", archived.Status)
	}
}

func TestPurgeHandler_DeletesObjectsThenRow(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	deletedAt := now.Add(-31 * 24 * time.Hour)
	e := &Event{ID: uuid.New(), UserID: uuid.New(), Name: "Old", Status: StatusArchived,
		StorageBytes: 2048, DeletedAt: &deletedAt}
	repo.events[e.ID] = e
	repo.purgeKeys[e.ID] = []string{"originals/a.jpg", "thumbnails/a.webp"}

	store := &fakeDeleter{}
	h := PurgeHandler(repo, store, 30*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(store.deleted) != 2 {
		t.Fatalf("deleted = %v", store.deleted)
	}
	if _, ok := repo.events[e.ID]; ok {
		t.Fatal("event row still present")
	}
	if repo.purgeCalls != 1 {
		t.Fatalf("purge calls = %d", repo.purgeCalls)
	}

	// Second run has nothing to do.
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if repo.purgeCalls != 1 {
		t.Fatalf("purge calls = %d after rerun", repo.purgeCalls)
	}
}

func TestPurgeHandler_PurgesExpiredPastGrace(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	expires := now.Add(-31 * 24 * time.Hour)
	e := &Event{ID: uuid.New(), UserID: uuid.New(), Status: StatusExpired, ExpiresAt: &expires}
	repo.events[e.ID] = e

	h := PurgeHandler(repo, &fakeDeleter{}, 30*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.events[e.ID]; ok {
		t.Fatal("expired event should be purged")
	}
}

func TestPurgeHandler_SkipsWithinGrace(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	deletedAt := now.Add(-5 * 24 * time.Hour)
	e := &Event{ID: uuid.New(), UserID: uuid.New(), Status: StatusArchived, DeletedAt: &deletedAt}
	repo.events[e.ID] = e

	h := PurgeHandler(repo, &fakeDeleter{}, 30*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.events[e.ID]; !ok {
		t.Fatal("event within grace must not be purged")
	}
}

func TestPurgeHandler_ToleratesMissingObject(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	deletedAt := now.Add(-31 * 24 * time.Hour)
	e := &Event{ID: uuid.New(), UserID: uuid.New(), Status: StatusArchived, DeletedAt: &deletedAt}
	repo.events[e.ID] = e
	repo.purgeKeys[e.ID] = []string{"missing", "present"}

	store := &fakeDeleter{failKey: "missing", err: r2.ErrNotFound}
	h := PurgeHandler(repo, store, 30*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.events[e.ID]; ok {
		t.Fatal("event should be purged when an object is already gone")
	}
}

func TestPurgeHandler_ObjectErrorKeepsRow(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	repo := newFakeRepo()
	deletedAt := now.Add(-31 * 24 * time.Hour)
	e := &Event{ID: uuid.New(), UserID: uuid.New(), Status: StatusArchived, DeletedAt: &deletedAt}
	repo.events[e.ID] = e
	repo.purgeKeys[e.ID] = []string{"a"}

	store := &fakeDeleter{failKey: "a", err: errors.New("storage down")}
	h := PurgeHandler(repo, store, 30*24*time.Hour, func() time.Time { return now }, discardLogger())
	if err := h(context.Background(), nil); err == nil {
		t.Fatal("expected storage error")
	}
	if _, ok := repo.events[e.ID]; !ok {
		t.Fatal("event must survive a failed object delete")
	}
	if repo.purgeCalls != 0 {
		t.Fatalf("purge calls = %d", repo.purgeCalls)
	}
}
