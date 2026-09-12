package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type fakeRepo struct {
	events   map[uuid.UUID]*Event
	settings map[uuid.UUID]*Settings
	slugs    map[string]uuid.UUID
	now      time.Time
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:   map[uuid.UUID]*Event{},
		settings: map[uuid.UUID]*Settings{},
		slugs:    map[string]uuid.UUID{},
		now:      time.Now(),
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

type fixedLimits struct {
	max int
	err error
}

func (l fixedLimits) MaxActiveEvents(ctx context.Context, userID string) (int, error) {
	return l.max, l.err
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

func TestCreate_ActiveLimitReached(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, fixedLimits{max: 1})
	userID := uuid.New()

	_, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "First", Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.Create(context.Background(), userID, CreateParams{Name: "Second", Status: StatusActive})
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
	// Upcoming is not limited.
	if _, _, err := svc.Create(context.Background(), userID, CreateParams{Name: "Third", Status: StatusUpcoming}); err != nil {
		t.Fatalf("upcoming should not be limited: %v", err)
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
	if _, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "First", Status: StatusActive}); err != nil {
		t.Fatal(err)
	}
	second, _, err := svc.Create(context.Background(), owner, CreateParams{Name: "Second"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Transition(context.Background(), owner, second.ID, StatusActive)
	if got := appErrCode(t, err); got != "PLAN_LIMIT_REACHED" {
		t.Fatalf("got %s", got)
	}
}
