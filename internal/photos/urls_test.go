package photos

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubPresigner struct {
	key string
	ttl time.Duration
	err error
}

func (s *stubPresigner) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	s.key = key
	s.ttl = ttl
	if s.err != nil {
		return "", s.err
	}
	return "https://signed.example/" + key, nil
}

func photoWithKeys() *Photo {
	t, m, o := "t.webp", "m.webp", "o.webp"
	return &Photo{
		ID:           uuid.New(),
		StorageKey:   "orig/a.jpg",
		ThumbnailKey: &t,
		MediumKey:    &m,
		OptimizedKey: &o,
	}
}

func TestSignedURLGenerator_VariantKeyMapping(t *testing.T) {
	cases := map[string]string{
		"thumbnail": "t.webp",
		"medium":    "m.webp",
		"large":     "o.webp",
		"original":  "orig/a.jpg",
	}
	for variant, want := range cases {
		sp := &stubPresigner{}
		g := NewSignedURLGenerator(sp, 5*time.Minute)
		res, err := g.URL(context.Background(), photoWithKeys(), variant)
		require.NoError(t, err, variant)
		require.Equal(t, want, sp.key)
		require.Equal(t, 300, res.ExpiresIn)
	}
}

func TestSignedURLGenerator_InvalidVariant(t *testing.T) {
	g := NewSignedURLGenerator(&stubPresigner{}, time.Minute)
	_, err := g.URL(context.Background(), photoWithKeys(), "huge")
	require.ErrorIs(t, err, ErrVariantUnavailable)
}

func TestSignedURLGenerator_MissingDerivative(t *testing.T) {
	p := photoWithKeys()
	p.MediumKey = nil
	g := NewSignedURLGenerator(&stubPresigner{}, time.Minute)
	_, err := g.URL(context.Background(), p, "medium")
	require.ErrorIs(t, err, ErrVariantUnavailable)
}

func TestSignedURLGenerator_StoreError(t *testing.T) {
	boom := errors.New("boom")
	g := NewSignedURLGenerator(&stubPresigner{err: boom}, time.Minute)
	_, err := g.URL(context.Background(), photoWithKeys(), "large")
	require.ErrorIs(t, err, boom)
}
