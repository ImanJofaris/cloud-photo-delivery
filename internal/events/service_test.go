package events

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type fakeRepo struct {
	events      map[uuid.UUID]*Event
	settings    map[uuid.UUID]*Settings
	slugs       map[string]uuid.UUID
	warned      map[uuid.UUID]time.Time
	ownerEmail  string
	now         time.Time
	purgeCalls  int
	purgeKeys   map[uuid.UUID][]string
	purgeDelete map[uuid.UUID]bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:      map[uuid.UUID]*Event{},
		settings:    map[uuid.UUID]*Settings{},
		slugs:       map[string]uuid.UUID{},
		warned:      map[uuid.UUID]time.Time{},
		ownerEmail:  "owner@example.com",
		now:         time.Now(),
		purgeKeys:   map[uuid.UUID][]string{},
		purgeDelete: map[uuid.UUID]bool{},
	}
}

func slugKey(userID uuid.UUID, slug string) string { return userID.String() + "/" + slug }

func (f *fakeRepo) Create(ctx context.Context, in CreateInput) (*Event, *Settings, error) {
	if _, ok := f.slugs[slugKey(in.UserID, in.Slug)]; ok {
		return nil, nil, ErrSlugTaken
	}
	id := uuid.New()
	e := &Event{
		ID: id, UserID: in.UserID, Name: in.Name, Slug: in.Slug,
		ClientName: in.ClientName, ClientEmail: in.ClientEmail,
		Location: in.Location, Description: in.Description,
		EventDate: in.EventDate, Status: in.Status, ExpiresAt: in.ExpiresAt,
		CreatedAt: f.now, UpdatedAt: f.now,
	}
	s := in.Settings
	s.EventID = id
	s.UpdatedAt = f.now
	f.events[id] = e
	f.settings[id] = &s
	f.slugs[slugKey(in.UserID, in.Slug)] = id
	return e, &s, nil
}

func (f *fakeRepo) GetByID(ctx context.Context, userID, id uuid.UUID) (*Event, error) {
	e, ok := f.events[id]
	if !ok || e.UserID != userID || e.DeletedAt != nil {
		return nil, ErrNotFound
	}
	return e, nil
}

func (f *fakeRepo) List(ctx context.Context, userID uuid.UUID, filt ListFilter) ([]*Event, error) {
	var out []*Event
	for _, e := range f.events {
		if e.UserID != userID || e.DeletedAt != nil {
			continue
		}
		if filt.Status != nil && e.Status != *filt.Status {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeRepo) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (*Event, error) {
	e, err := f.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		e.Name = *in.Name
	}
	if in.ClientEmail != nil {
		e.ClientEmail = *in.ClientEmail
	}
	if in.ClearDate {
		e.EventDate = nil
	} else if in.EventDate != nil {
		e.EventDate = in.EventDate
	}
	if in.ExpiresAt != nil {
		e.ExpiresAt = in.ExpiresAt
	} else if in.ClearExpiry {
		e.ExpiresAt = nil
	}
	return e, nil
}

func (f *fakeRepo) SetStatus(ctx context.Context, userID, id uuid.UUID, status Status) (*Event, error) {
	e, err := f.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	e.Status = status
	return e, nil
}

func (f *fakeRepo) SoftDelete(ctx context.Context, userID, id uuid.UUID) error {
	e, err := f.GetByID(ctx, userID, id)
	if err != nil {
		return err
	}
	now := f.now
	e.DeletedAt = &now
	return nil
}

func (f *fakeRepo) CountByStatus(ctx context.Context, userID uuid.UUID, status Status) (int, error) {
	n := 0
	for _, e := range f.events {
		if e.UserID == userID && e.DeletedAt == nil && e.Status == status {
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) CountLiveEvents(ctx context.Context, userID uuid.UUID) (int, error) {
	n := 0
	for _, e := range f.events {
		if e.UserID != userID || e.DeletedAt != nil {
			continue
		}
		switch e.Status {
		case StatusUpcoming, StatusActive, StatusCompleted:
			n++
		}
	}
	return n, nil
}

func (f *fakeRepo) SlugExists(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	_, ok := f.slugs[slugKey(userID, slug)]
	return ok, nil
}

func (f *fakeRepo) GetSettings(ctx context.Context, userID, eventID uuid.UUID) (*Settings, error) {
	if _, err := f.GetByID(ctx, userID, eventID); err != nil {
		return nil, err
	}
	s, ok := f.settings[eventID]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (f *fakeRepo) UpdateSettings(ctx context.Context, userID, eventID uuid.UUID, s Settings) (*Settings, error) {
	if _, err := f.GetByID(ctx, userID, eventID); err != nil {
		return nil, err
	}
	s.EventID = eventID
	s.UpdatedAt = f.now
	f.settings[eventID] = &s
	return &s, nil
}

func (f *fakeRepo) Extend(ctx context.Context, userID, id uuid.UUID, expiresAt time.Time, reactivate bool) (*Event, error) {
	e, err := f.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	e.ExpiresAt = &expiresAt
	if reactivate {
		e.Status = StatusActive
	}
	delete(f.warned, id)
	return e, nil
}

func (f *fakeRepo) ListDueExpiry(ctx context.Context, now time.Time, limit int) ([]*Event, error) {
	var out []*Event
	for _, e := range f.events {
		if e.DeletedAt != nil || e.ExpiresAt == nil || e.ExpiresAt.After(now) {
			continue
		}
		if e.Status != StatusUpcoming && e.Status != StatusActive && e.Status != StatusCompleted {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExpiresAt.Before(*out[j].ExpiresAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeRepo) ListExpiryWarnings(ctx context.Context, now, horizon time.Time, limit int) ([]ExpiryWarning, error) {
	var out []ExpiryWarning
	for _, e := range f.events {
		if e.DeletedAt != nil || e.ExpiresAt == nil {
			continue
		}
		if !e.ExpiresAt.After(now) || e.ExpiresAt.After(horizon) {
			continue
		}
		if e.Status != StatusUpcoming && e.Status != StatusActive && e.Status != StatusCompleted {
			continue
		}
		if _, ok := f.warned[e.ID]; ok {
			continue
		}
		out = append(out, ExpiryWarning{
			EventID: e.ID, UserID: e.UserID, Name: e.Name,
			OwnerEmail: f.ownerEmail, ExpiresAt: *e.ExpiresAt,
		})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeRepo) MarkExpired(ctx context.Context, id uuid.UUID) error {
	e, ok := f.events[id]
	if !ok || e.DeletedAt != nil {
		return nil
	}
	switch e.Status {
	case StatusUpcoming, StatusActive, StatusCompleted:
		e.Status = StatusExpired
	}
	return nil
}

func (f *fakeRepo) MarkExpiryWarned(ctx context.Context, id uuid.UUID, warnedAt time.Time) error {
	f.warned[id] = warnedAt
	return nil
}

func (f *fakeRepo) ListPurgeable(ctx context.Context, cutoff time.Time, limit int) ([]PurgeCandidate, error) {
	var out []PurgeCandidate
	for _, e := range f.events {
		softDue := e.DeletedAt != nil && !e.DeletedAt.After(cutoff)
		expiredDue := e.Status == StatusExpired && e.ExpiresAt != nil && !e.ExpiresAt.After(cutoff)
		if !softDue && !expiredDue {
			continue
		}
		out = append(out, PurgeCandidate{
			EventID: e.ID, UserID: e.UserID, Name: e.Name, StorageBytes: e.StorageBytes,
		})
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeRepo) PurgeKeys(ctx context.Context, eventID uuid.UUID) ([]string, error) {
	return append([]string(nil), f.purgeKeys[eventID]...), nil
}

func (f *fakeRepo) PurgeEvent(ctx context.Context, eventID uuid.UUID) (bool, error) {
	if _, ok := f.events[eventID]; !ok {
		return false, nil
	}
	delete(f.events, eventID)
	delete(f.settings, eventID)
	delete(f.purgeKeys, eventID)
	f.purgeCalls++
	f.purgeDelete[eventID] = true
	return true, nil
}

type fixedLimits struct {
	max                int
	retention          int
	err                error
	noAPI              bool
	noBranding         bool
	noOriginalDownload bool
}

func (l fixedLimits) MaxEvents(ctx context.Context, userID string) (int, error) {
	return l.max, l.err
}

func (l fixedLimits) MaxPhotosPerEvent(ctx context.Context, userID string) (int, error) {
	return 0, l.err
}

func (l fixedLimits) MaxStorageBytes(ctx context.Context, userID string) (int64, error) {
	return 0, l.err
}

func (l fixedLimits) RetentionDays(ctx context.Context, userID string) (int, error) {
	return l.retention, l.err
}

func (l fixedLimits) APIAccess(ctx context.Context, userID string) (bool, error) {
	return !l.noAPI, l.err
}

func (l fixedLimits) BrandingEnabled(ctx context.Context, userID string) (bool, error) {
	return !l.noBranding, l.err
}

func (l fixedLimits) OriginalDownloads(ctx context.Context, userID string) (bool, error) {
	return !l.noOriginalDownload, l.err
}

func testHash(pw string) (string, error) { return "hashed:" + pw, nil }

func newTestService(repo Repository, l PlanLimits) *Service {
	svc := NewService(repo, l, testHash)
	svc.SetClock(func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) })
	return svc
}

func appErrCode(t *testing.T, err error) string {
	t.Helper()
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		t.Fatalf("expected apperr, got %v", err)
	}
	return ae.Code
}

func TestCreate_RequiresName(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "   "})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestCreate_InvalidClientEmail(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party", ClientEmail: "not-an-email"})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestCreate_GeneratesSlug(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	e, s, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Summer Party"})
	if err != nil {
		t.Fatal(err)
	}
	if e.Slug != "summer-party" {
		t.Fatalf("slug %q", e.Slug)
	}
	if e.Status != StatusUpcoming {
		t.Fatalf("default status %q", e.Status)
	}
	if s.Visibility != VisibilityPublic {
		t.Fatalf("default visibility %q", s.Visibility)
	}
	if !s.AllowDownload {
		t.Fatal("default allowDownload should be true")
	}
}

func TestCreate_ExplicitAllowDownloadFalseSticks(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	no := false
	_, s, err := svc.Create(context.Background(), uuid.New(), CreateParams{
		Name:     "Party",
		Settings: SettingsInput{AllowDownload: &no},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.AllowDownload {
		t.Fatal("explicit allowDownload=false must stick")
	}
}

func TestCreate_UniqueSlugWithinTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	userID := uuid.New()

	e1, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	e2, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	e3, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	if e1.Slug != "party" || e2.Slug != "party-2" || e3.Slug != "party-3" {
		t.Fatalf("slugs %q %q %q", e1.Slug, e2.Slug, e3.Slug)
	}
}

func TestCreate_SameSlugAcrossTenantsAllowed(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})

	a, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Slug != "party" || b.Slug != "party" {
		t.Fatalf("slugs %q %q", a.Slug, b.Slug)
	}
}

func TestCreate_LiveEventLimitReached(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{max: 1})
	userID := uuid.New()

	first, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "First"})
	if err != nil {
		t.Fatal(err)
	}
	// Upcoming events are reachable, so they occupy a slot too.
	_, _, err = svc.Create(context.Background(), userID, CreateParams{Name: "Second"})
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
	// Archiving frees the slot.
	if _, err := svc.Archive(context.Background(), userID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Third"}); err != nil {
		t.Fatalf("archive should free a slot: %v", err)
	}
}

func TestCreate_InvalidStatus(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "X", Status: Status("bogus")})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestGet_NotFoundForOtherTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Get(context.Background(), uuid.New(), e.ID); appErrCode(t, err) != "EVENT_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
	// Owner still sees it.
	if _, err := svc.Get(context.Background(), owner, e.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUpdate_NotFoundForOtherTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	name := "Renamed"
	if _, err := svc.Update(context.Background(), uuid.New(), e.ID, UpdateParams{Name: &name}); appErrCode(t, err) != "EVENT_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}

func TestUpdate_RequiresNonEmptyName(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	empty := "  "
	if _, err := svc.Update(context.Background(), owner, e.ID, UpdateParams{Name: &empty}); appErrCode(t, err) != "VALIDATION_ERROR" {
		t.Fatalf("got %v", err)
	}
}

func TestUpdate_ExpiryBeyondRetentionRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	beyond := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if _, err := svc.Update(context.Background(), owner, e.ID, UpdateParams{ExpiresAt: &beyond}); appErrCode(t, err) != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %v", err)
	}
}

func TestUpdate_ClearExpiryResetsToRetention(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	updated, err := svc.Update(context.Background(), owner, e.ID, UpdateParams{ClearExpiry: true})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", updated.ExpiresAt, want)
	}
}

func TestUpdate_ClearExpiryUnlimitedPlanStaysNull(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	updated, err := svc.Update(context.Background(), owner, e.ID, UpdateParams{ClearExpiry: true})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ExpiresAt != nil {
		t.Fatalf("expiresAt = %v, want nil", updated.ExpiresAt)
	}
}

func TestDelete_NotFoundForOtherTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	if appErrCode(t, svc.Delete(context.Background(), uuid.New(), e.ID)) != "EVENT_NOT_FOUND" {
		t.Fatal("expected EVENT_NOT_FOUND")
	}
}

func TestTransition_Valid(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	active, err := svc.Transition(context.Background(), owner, e.ID, StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if active.Status != StatusActive {
		t.Fatalf("got %q", active.Status)
	}
	completed, err := svc.Transition(context.Background(), owner, e.ID, StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != StatusCompleted {
		t.Fatalf("got %q", completed.Status)
	}
}

func TestTransition_Invalid(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	// upcoming -> completed is not allowed
	_, err := svc.Transition(context.Background(), owner, e.ID, StatusCompleted)
	if got := appErrCode(t, err); got != "INVALID_STATUS_TRANSITION" {
		t.Fatalf("got %s", got)
	}
}

func TestArchive(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	archived, err := svc.Archive(context.Background(), owner, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != StatusArchived {
		t.Fatalf("got %q", archived.Status)
	}
	// archived is terminal
	if _, err := svc.Transition(context.Background(), owner, e.ID, StatusActive); appErrCode(t, err) != "INVALID_STATUS_TRANSITION" {
		t.Fatalf("expected terminal, got %v", err)
	}
}

func TestSettings_PasswordRequiresPassword(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	v := VisibilityPassword
	_, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Visibility: &v})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestSettings_PasswordSetsVisibility(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	pw := "secret123"
	s, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Password: &pw})
	if err != nil {
		t.Fatal(err)
	}
	if s.Visibility != VisibilityPassword {
		t.Fatalf("visibility %q", s.Visibility)
	}
	if s.PasswordHash == "" || s.PasswordHash == pw {
		t.Fatalf("password not hashed: %q", s.PasswordHash)
	}
}

func TestSettings_PrivateDisablesDownloads(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	v := VisibilityPrivate
	s, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Visibility: &v})
	if err != nil {
		t.Fatal(err)
	}
	if s.AllowDownload || s.AllowOriginalDownload {
		t.Fatal("private must disable downloads")
	}
}

func TestSettings_OriginalRequiresDownload(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	no := false
	yes := true
	s, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{AllowDownload: &no, AllowOriginalDownload: &yes})
	if err != nil {
		t.Fatal(err)
	}
	if s.AllowOriginalDownload {
		t.Fatal("original download must be off when download is off")
	}
}

func TestSettings_PasswordSetsChangedAt(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	pw := "secret123"
	s, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Password: &pw})
	if err != nil {
		t.Fatal(err)
	}
	if s.PasswordChangedAt == nil {
		t.Fatal("expected password_changed_at to be set")
	}
	want := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !s.PasswordChangedAt.Equal(want) {
		t.Fatalf("password_changed_at = %v, want %v", s.PasswordChangedAt, want)
	}
}

func TestSettings_SamePasswordDoesNotBumpChangedAt(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	pw := "secret123"
	first, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Password: &pw})
	if err != nil {
		t.Fatal(err)
	}
	firstAt := *first.PasswordChangedAt

	// Move the clock forward and update unrelated settings.
	svc.SetClock(func() time.Time { return time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC) })
	no := false
	second, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{AllowDownload: &no})
	if err != nil {
		t.Fatal(err)
	}
	if second.PasswordChangedAt == nil || !second.PasswordChangedAt.Equal(firstAt) {
		t.Fatalf("password_changed_at changed unexpectedly: %v", second.PasswordChangedAt)
	}
}

func TestSettings_ClearingPasswordBumpsChangedAt(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, _ := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})

	pw := "secret123"
	if _, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Password: &pw}); err != nil {
		t.Fatal(err)
	}

	svc.SetClock(func() time.Time { return time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC) })
	empty := ""
	pub := VisibilityPublic
	s, err := svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{Password: &empty, Visibility: &pub})
	if err != nil {
		t.Fatal(err)
	}
	if s.PasswordHash != "" {
		t.Fatal("expected password hash cleared")
	}
	if s.Visibility != VisibilityPublic {
		t.Fatalf("visibility = %q, want public", s.Visibility)
	}
	want := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	if s.PasswordChangedAt == nil || !s.PasswordChangedAt.Equal(want) {
		t.Fatalf("password_changed_at = %v, want %v", s.PasswordChangedAt, want)
	}
}

func TestList_InvalidStatusRejected(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	bad := Status("nope")
	_, err := svc.List(context.Background(), uuid.New(), ListParams{Status: &bad})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestList_InvalidCursorRejected(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	_, err := svc.List(context.Background(), uuid.New(), ListParams{Cursor: "!!!not-base64"})
	if got := appErrCode(t, err); got != "VALIDATION_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestTransition_ActiveLimitReached(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{max: 1})
	owner := uuid.New()
	if _, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "First"}); err != nil {
		t.Fatal(err)
	}
	// An expired event does not occupy a slot, so it can exist alongside the
	// live one; reactivating it must respect the limit.
	secondID := uuid.New()
	repo.events[secondID] = &Event{ID: secondID, UserID: owner, Name: "Second", Status: StatusExpired}
	repo.settings[secondID] = &Settings{EventID: secondID}
	_, err := svc.Transition(context.Background(), owner, secondID, StatusActive)
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestCreate_DerivesExpiryFromPlan(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	e, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
	if e.ExpiresAt == nil || !e.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", e.ExpiresAt, want)
	}
}

func TestCreate_ExplicitExpiryWithinRetention(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	explicit := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	e, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party", ExpiresAt: &explicit})
	if err != nil {
		t.Fatal(err)
	}
	if e.ExpiresAt == nil || !e.ExpiresAt.Equal(explicit) {
		t.Fatalf("expiresAt = %v, want %v", e.ExpiresAt, explicit)
	}
}

func TestCreate_ExplicitExpiryBeyondRetentionRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	explicit := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party", ExpiresAt: &explicit})
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestCreate_ZeroRetentionStaysNull(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	e, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	if e.ExpiresAt != nil {
		t.Fatalf("expiresAt = %v, want nil", e.ExpiresAt)
	}
}

func TestCreate_RetentionErrorMapsToInternal(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{err: errors.New("db down")})
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{Name: "Party"})
	if got := appErrCode(t, err); got != "INTERNAL_ERROR" {
		t.Fatalf("got %s", got)
	}
}

func TestCreate_OriginalDownloadsGated(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{noOriginalDownload: true})
	yes := true
	_, _, err := svc.Create(context.Background(), uuid.New(), CreateParams{
		Name:     "Party",
		Settings: SettingsInput{AllowOriginalDownload: &yes},
	})
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestUpdateSettings_OriginalDownloadsGated(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{noOriginalDownload: true})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	yes := true
	_, err = svc.UpdateSettings(context.Background(), owner, e.ID, SettingsInput{AllowOriginalDownload: &yes})
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestExtend_Validation(t *testing.T) {
	svc := newTestService(newFakeRepo(), fixedLimits{})
	for _, days := range []int{0, -1, maxExtendDays + 1} {
		if _, err := svc.Extend(context.Background(), uuid.New(), uuid.New(), days); appErrCode(t, err) != "VALIDATION_ERROR" {
			t.Fatalf("days=%d got %v", days, err)
		}
	}
}

func TestExtend_ExtendsFromLaterOfNowAndExpiry(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()

	future := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	repo.events[e.ID].ExpiresAt = &future

	updated, err := svc.Extend(context.Background(), owner, e.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", updated.ExpiresAt, want)
	}

	past := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	repo.events[e.ID].ExpiresAt = &past
	updated, err = svc.Extend(context.Background(), owner, e.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(want) {
		t.Fatalf("expiresAt = %v, want %v", updated.ExpiresAt, want)
	}
}

func TestExtend_ReactivatesExpired(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	expired, err := svc.Transition(context.Background(), owner, e.ID, StatusExpired)
	if err != nil {
		t.Fatal(err)
	}
	if expired.Status != StatusExpired {
		t.Fatalf("status = %q", expired.Status)
	}

	updated, err := svc.Extend(context.Background(), owner, e.ID, 7)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != StatusActive {
		t.Fatalf("status = %q, want active", updated.Status)
	}
	if updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("expiresAt = %v", updated.ExpiresAt)
	}
}

func TestExtend_ReactivationEnforcesActiveLimit(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{max: 1})
	owner := uuid.New()
	a, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Transition(context.Background(), owner, a.ID, StatusExpired); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "B"}); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Extend(context.Background(), owner, a.ID, 7)
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestExtend_BeyondRetentionRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{retention: 7})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	// now + 7 + 7 days would exceed the plan retention window.
	_, err = svc.Extend(context.Background(), owner, e.ID, 30)
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}

func TestExtend_ArchivedRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Archive(context.Background(), owner, e.ID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Extend(context.Background(), owner, e.ID, 7)
	if got := appErrCode(t, err); got != "INVALID_STATUS_TRANSITION" {
		t.Fatalf("got %s", got)
	}
}

func TestExtend_NotFoundForOtherTenant(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{})
	owner := uuid.New()
	e, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Party"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Extend(context.Background(), uuid.New(), e.ID, 7); appErrCode(t, err) != "EVENT_NOT_FOUND" {
		t.Fatalf("got %v", err)
	}
}
