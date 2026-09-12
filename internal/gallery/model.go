package gallery

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
)

type VisibleEvent struct {
	Event    *events.Event
	Settings *events.Settings
}

type PhotoPage struct {
	Items      []*photos.Photo
	NextCursor string
}

type Variant string

const (
	VariantThumbnail Variant = "thumbnail"
	VariantMedium    Variant = "medium"
	VariantLarge     Variant = "large"
	VariantOriginal  Variant = "original"
)

func (v Variant) Valid() bool {
	switch v {
	case VariantThumbnail, VariantMedium, VariantLarge, VariantOriginal:
		return true
	default:
		return false
	}
}

func (v Variant) IsOriginal() bool { return v == VariantOriginal }

// VariantsFor returns the derivative variants available for a READY photo.
func VariantsFor(p *photos.Photo) []Variant {
	out := make([]Variant, 0, 3)
	if p.ThumbnailKey != nil {
		out = append(out, VariantThumbnail)
	}
	if p.MediumKey != nil {
		out = append(out, VariantMedium)
	}
	if p.OptimizedKey != nil {
		out = append(out, VariantLarge)
	}
	return out
}

type UnlockResult struct {
	Token     string
	ExpiresIn int
}

// Repository is the read-only gallery data access layer.
type Repository interface {
	EventBySlug(ctx context.Context, slug string) (*events.Event, *events.Settings, error)
	GetSettingsByEventID(ctx context.Context, eventID uuid.UUID) (*events.Settings, error)
	ListReadyPhotos(ctx context.Context, eventID uuid.UUID, cursor *Cursor, limit int) ([]*photos.Photo, error)
	ReadyPhoto(ctx context.Context, eventID, photoID uuid.UUID) (*photos.Photo, error)
}

type Clock func() time.Time
