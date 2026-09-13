package gallery

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
)

type VisibleEvent struct {
	Event    *events.Event
	Settings *events.Settings
	Branding *users.BrandingView
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

// Visitor carries the request attributes used for anonymous analytics. No raw
// IP or user agent is persisted; the recorder hashes them.
type Visitor struct {
	IP        string
	UserAgent string
	QRScan    bool
}

// AnalyticsRecorder records public gallery activity. The gallery treats
// failures as non-fatal so analytics can never break a gallery read.
type AnalyticsRecorder interface {
	RecordView(ctx context.Context, eventID uuid.UUID, ip, userAgent string, qrScan bool) error
	RecordDownload(ctx context.Context, eventID uuid.UUID) error
}

// Repository is the read-only gallery data access layer.
type Repository interface {
	EventBySlug(ctx context.Context, slug string) (*events.Event, *events.Settings, error)
	GetSettingsByEventID(ctx context.Context, eventID uuid.UUID) (*events.Settings, error)
	ListReadyPhotos(ctx context.Context, eventID uuid.UUID, cursor *Cursor, limit int) ([]*photos.Photo, error)
	ReadyPhoto(ctx context.Context, eventID, photoID uuid.UUID) (*photos.Photo, error)
}

// BrandingProvider resolves an operator's branding for the gallery DTO. A
// missing branding row yields an empty view, not an error.
type BrandingProvider interface {
	Get(ctx context.Context, userID uuid.UUID) (*users.BrandingView, error)
}

type Clock func() time.Time
