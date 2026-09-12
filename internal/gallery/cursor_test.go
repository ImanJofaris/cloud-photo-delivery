package gallery

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCursor_RoundTrip(t *testing.T) {
	in := Cursor{CreatedAt: time.Now().UTC().Truncate(time.Nanosecond), ID: uuid.New()}
	enc := EncodeCursor(in)
	out, err := DecodeCursor(enc)
	require.NoError(t, err)
	require.True(t, in.CreatedAt.Equal(out.CreatedAt))
	require.Equal(t, in.ID, out.ID)
}

func TestCursor_EmptyIsNil(t *testing.T) {
	c, err := DecodeCursor("")
	require.NoError(t, err)
	require.Nil(t, c)
}

func TestCursor_InvalidRejected(t *testing.T) {
	for _, s := range []string{"not-base64!!", "aGVsbG8", "MjAyNi0wMS0wMXw", "MjAyNi0wMS0wMXxub3QtdXVpZA"} {
		_, err := DecodeCursor(s)
		require.ErrorIs(t, err, ErrInvalidCursor, "input %q", s)
	}
}

func TestNormalizeLimit(t *testing.T) {
	require.Equal(t, DefaultLimit, NormalizeLimit(0))
	require.Equal(t, DefaultLimit, NormalizeLimit(-5))
	require.Equal(t, 10, NormalizeLimit(10))
	require.Equal(t, MaxLimit, NormalizeLimit(1000))
}
