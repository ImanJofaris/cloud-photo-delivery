package photos

import (
	"context"
	"strings"
	"time"
)

type GetPresigner interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}

// URLResult is a signed URL and the number of seconds until it expires.
type URLResult struct {
	URL       string
	ExpiresIn int
}

// SignedURLGenerator resolves a photo variant to a short-lived signed URL.
type SignedURLGenerator struct {
	store GetPresigner
	ttl   time.Duration
}

func NewSignedURLGenerator(store GetPresigner, ttl time.Duration) *SignedURLGenerator {
	return &SignedURLGenerator{store: store, ttl: ttl}
}

func (g *SignedURLGenerator) TTL() time.Duration { return g.ttl }

// URL returns a signed URL for the given variant. The caller is responsible for
// the visibility/download checks; this only maps variant -> storage key.
func (g *SignedURLGenerator) URL(ctx context.Context, p *Photo, variant string) (*URLResult, error) {
	key, ok := variantKey(p, variant)
	if !ok {
		return nil, ErrVariantUnavailable
	}
	url, err := g.store.PresignGet(ctx, key, g.ttl)
	if err != nil {
		return nil, err
	}
	return &URLResult{URL: url, ExpiresIn: int(g.ttl.Seconds())}, nil
}

var ErrVariantUnavailable = errVariantUnavailable{}

type errVariantUnavailable struct{}

func (errVariantUnavailable) Error() string { return "variant unavailable" }

func variantKey(p *Photo, variant string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(variant)) {
	case "thumbnail":
		return deref(p.ThumbnailKey)
	case "medium":
		return deref(p.MediumKey)
	case "large":
		return deref(p.OptimizedKey)
	case "original":
		return p.StorageKey, p.StorageKey != ""
	default:
		return "", false
	}
}

func deref(s *string) (string, bool) {
	if s == nil || *s == "" {
		return "", false
	}
	return *s, true
}
