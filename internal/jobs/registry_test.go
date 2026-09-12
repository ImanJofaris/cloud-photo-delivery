package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackoff(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{0, 1 * time.Second},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{9, 256 * time.Second},
		{10, 5 * time.Minute},
		{100, 5 * time.Minute},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, Backoff(tc.attempts), "attempts=%d", tc.attempts)
	}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := NewRegistry()
	require.Empty(t, r.Types())

	called := false
	r.Register("a", func(context.Context, []byte) error { called = true; return nil })
	r.Register("b", func(context.Context, []byte) error { return nil })
	require.Equal(t, []string{"a", "b"}, r.Types())

	h, ok := r.Lookup("a")
	require.True(t, ok)
	require.NoError(t, h(context.Background(), nil))
	require.True(t, called)

	_, ok = r.Lookup("missing")
	require.False(t, ok)
}

func TestRegistry_DispatchUnknownType(t *testing.T) {
	r := NewRegistry()
	err := r.Dispatch(context.Background(), &Job{Type: "nope"})
	require.ErrorIs(t, err, ErrUnknownJobType)
}

func TestRegistry_DispatchRoutesPayload(t *testing.T) {
	r := NewRegistry()
	var got []byte
	r.Register("echo", func(_ context.Context, p []byte) error { got = p; return nil })
	err := r.Dispatch(context.Background(), &Job{Type: "echo", Payload: []byte("hi")})
	require.NoError(t, err)
	require.Equal(t, []byte("hi"), got)
}

func TestRegistry_DispatchPropagatesHandlerError(t *testing.T) {
	r := NewRegistry()
	boom := errors.New("boom")
	r.Register("bad", func(context.Context, []byte) error { return boom })
	require.ErrorIs(t, r.Dispatch(context.Background(), &Job{Type: "bad"}), boom)
}
