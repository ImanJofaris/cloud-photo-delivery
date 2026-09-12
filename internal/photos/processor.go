package photos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/imaging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const (
	ThumbnailEdge = 400
	MediumEdge    = 1000
	OptimizedEdge = 2000
)

// Per-derivative WebP quality. Smaller derivatives can be pushed harder
// because their display size hides artifacts; the optimized large keeps more
// fidelity for full-screen viewing.
const (
	ThumbnailQuality = 80
	MediumQuality    = 82
	OptimizedQuality = 85
)

// Derivative kinds recorded on the photo row.
const (
	KindThumbnail = "thumbnail"
	KindMedium    = "medium"
	KindOptimized = "optimized"
)

// PhotoStore is the subset of object storage the processor needs.
type PhotoStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Put(ctx context.Context, key, contentType string, body []byte) error
}

// ProcessPayload is the job payload enqueued on upload completion.
type ProcessPayload struct {
	PhotoID uuid.UUID `json:"photoId"`
	EventID uuid.UUID `json:"eventId"`
}

// EventOwner resolves the owning user for an event so storage keys can be
// rebuilt. Backed by a scoped query in production.
type EventOwner func(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)

// Processor implements the image.process job handler. It downloads the
// original, validates it, generates derivatives and updates the photo row.
type Processor struct {
	repo    Repository
	store   PhotoStore
	decoder imaging.Decoder
	encoder imaging.Encoder
	resizer imaging.Resizer
	log     *slog.Logger
	owner   EventOwner
}

func NewProcessor(repo Repository, store PhotoStore, dec imaging.Decoder, enc imaging.Encoder, res imaging.Resizer, log *slog.Logger, owner EventOwner) *Processor {
	return &Processor{repo: repo, store: store, decoder: dec, encoder: enc, resizer: res, log: log, owner: owner}
}

// Handle is the jobs.Handler-compatible entrypoint.
func (p *Processor) Handle(ctx context.Context, payload []byte) error {
	var in ProcessPayload
	if err := json.Unmarshal(payload, &in); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if in.PhotoID == uuid.Nil {
		return errors.New("payload missing photoId")
	}

	photo, err := p.repo.GetByID(ctx, in.PhotoID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return err
	}
	if photo.Status == StatusReady {
		return nil
	}
	if photo.Status == StatusFailed {
		return errors.New("photo is in FAILED state")
	}

	userID, err := p.owner(ctx, photo.EventID)
	if err != nil {
		return fmt.Errorf("resolve event owner: %w", err)
	}

	original, err := p.store.Get(ctx, photo.StorageKey)
	if err != nil {
		return fmt.Errorf("download original: %w", err)
	}

	img, _, err := p.decoder.Decode(original)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	derivatives, err := p.generate(ctx, userID, photo, img)
	if err != nil {
		return err
	}

	if err := p.repo.MarkReady(ctx, photo.ID, width, height, derivatives); err != nil {
		return fmt.Errorf("mark ready: %w", err)
	}
	p.log.Info("photo processed", "photo_id", photo.ID, "width", width, "height", height)
	return nil
}

func (p *Processor) generate(ctx context.Context, userID uuid.UUID, photo *Photo, img image.Image) (Derivatives, error) {
	outputs := []struct {
		kind    string
		edge    int
		quality int
		key     string
	}{
		{KindThumbnail, ThumbnailEdge, ThumbnailQuality, r2.ThumbnailKey(userID, photo.EventID, photo.ID)},
		{KindMedium, MediumEdge, MediumQuality, r2.MediumKey(userID, photo.EventID, photo.ID)},
		{KindOptimized, OptimizedEdge, OptimizedQuality, r2.OptimizedKey(userID, photo.EventID, photo.ID)},
	}

	var out Derivatives
	for _, o := range outputs {
		resized := p.resizer.Resize(img, o.edge)
		encoded, err := p.encoder.Encode(resized, o.quality)
		if err != nil {
			return Derivatives{}, fmt.Errorf("encode %s: %w", o.kind, err)
		}
		if err := p.store.Put(ctx, o.key, "image/webp", encoded); err != nil {
			return Derivatives{}, fmt.Errorf("upload %s: %w", o.kind, err)
		}
		b := resized.Bounds()
		d := Derivative{Kind: o.kind, Key: o.key, Width: b.Dx(), Height: b.Dy()}
		switch o.kind {
		case KindThumbnail:
			out.Thumbnail = d
		case KindMedium:
			out.Medium = d
		case KindOptimized:
			out.Optimized = d
		}
	}
	return out, nil
}
