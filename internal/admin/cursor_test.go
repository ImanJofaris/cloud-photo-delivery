package admin

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCursorRoundTrip(t *testing.T) {
	in := Cursor{CreatedAt: time.Date(2026, 9, 14, 10, 30, 0, 0, time.UTC), ID: uuid.New()}
	out, err := DecodeCursor(EncodeCursor(in))
	require.NoError(t, err)
	require.True(t, in.CreatedAt.Equal(out.CreatedAt))
	require.Equal(t, in.ID, out.ID)
}

func TestDecodeCursor_EmptyIsNil(t *testing.T) {
	c, err := DecodeCursor("")
	require.NoError(t, err)
	require.Nil(t, c)
}

func TestDecodeCursor_Invalid(t *testing.T) {
	for _, raw := range []string{"not-base64!!", "Zm9v", "MjAyNi0wOS0xNHxub3QtYS11dWlk"} {
		_, err := DecodeCursor(raw)
		require.ErrorIs(t, err, ErrInvalidCursor, "raw=%s", raw)
	}
}

func TestNormalizeLimit(t *testing.T) {
	require.Equal(t, DefaultLimit, NormalizeLimit(0))
	require.Equal(t, DefaultLimit, NormalizeLimit(-5))
	require.Equal(t, 25, NormalizeLimit(25))
	require.Equal(t, MaxLimit, NormalizeLimit(MaxLimit+1))
}
