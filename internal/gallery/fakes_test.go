package gallery

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
)

type fakeRepo struct {
	mu       sync.Mutex
	events   map[string]*events.Event
	settings map[uuid.UUID]*events.Settings
	photos   map[uuid.UUID][]*photos.Photo
	err      error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		events:   map[string]*events.Event{},
		settings: map[uuid.UUID]*events.Settings{},
		photos:   map[uuid.UUID][]*photos.Photo{},
	}
}

func (f *fakeRepo) addEvent(e *events.Event, s *events.Settings) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events[e.Slug] = e
	f.settings[e.ID] = s
}

func (f *fakeRepo) EventBySlug(_ context.Context, slug string) (*events.Event, *events.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, nil, f.err
	}
	e, ok := f.events[slug]
	if !ok {
		return nil, nil, ErrNotFound
	}
	s, ok := f.settings[e.ID]
	if !ok {
		return nil, nil, ErrNotFound
	}
	ec, sc := *e, *s
	return &ec, &sc, nil
}

func (f *fakeRepo) GetSettingsByEventID(_ context.Context, eventID uuid.UUID) (*events.Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.settings[eventID]
	if !ok {
		return nil, ErrNotFound
	}
	sc := *s
	return &sc, nil
}

func (f *fakeRepo) ListReadyPhotos(_ context.Context, eventID uuid.UUID, cursor *Cursor, limit int) ([]*photos.Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	list := append([]*photos.Photo(nil), f.photos[eventID]...)
	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].ID.String() > list[j].ID.String()
		}
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	out := make([]*photos.Photo, 0, limit)
	for _, p := range list {
		if p.Status != photos.StatusReady {
			continue
		}
		if cursor != nil {
			if p.CreatedAt.After(cursor.CreatedAt) {
				continue
			}
			if p.CreatedAt.Equal(cursor.CreatedAt) && p.ID.String() >= cursor.ID.String() {
				continue
			}
		}
		out = append(out, p)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRepo) ReadyPhoto(_ context.Context, eventID, photoID uuid.UUID) (*photos.Photo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.photos[eventID] {
		if p.ID == photoID && p.Status == photos.StatusReady {
			cp := *p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

type fakePresigner struct {
	url string
	err error
	ttl time.Duration
	key string
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	f.key = key
	f.ttl = ttl
	if f.err != nil {
		return "", f.err
	}
	if f.url != "" {
		return f.url, nil
	}
	return "https://example.test/" + key, nil
}

func readyPhoto(eventID uuid.UUID, createdAt time.Time) *photos.Photo {
	t := "thumb/" + uuid.NewString()
	m := "medium/" + uuid.NewString()
	o := "opt/" + uuid.NewString()
	w, h := 2000, 1333
	return &photos.Photo{
		ID:           uuid.New(),
		EventID:      eventID,
		StorageKey:   "orig/" + uuid.NewString(),
		ThumbnailKey: &t,
		MediumKey:    &m,
		OptimizedKey: &o,
		Status:       photos.StatusReady,
		Width:        &w,
		Height:       &h,
		CreatedAt:    createdAt,
	}
}

func publicEvent(slug string) (*events.Event, *events.Settings) {
	id := uuid.New()
	e := &events.Event{ID: id, Name: "Wedding", Slug: slug, PhotoCount: 3}
	s := &events.Settings{EventID: id, Visibility: events.VisibilityPublic, AllowDownload: true}
	return e, s
}

type fakeBrandingProvider struct {
	view   *users.BrandingView
	err    error
	userID uuid.UUID
}

func (f *fakeBrandingProvider) Get(_ context.Context, userID uuid.UUID) (*users.BrandingView, error) {
	f.userID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.view, nil
}
