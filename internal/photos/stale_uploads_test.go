package photos

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeStaleRepo struct {
	cutoff time.Time
	failed int
	err    error
}

func (f *fakeStaleRepo) FailStaleUploads(_ context.Context, cutoff time.Time) (int, error) {
	f.cutoff = cutoff
	return f.failed, f.err
}

func TestStaleUploadHandler_UsesCutoff(t *testing.T) {
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	repo := &fakeStaleRepo{}
	handler := StaleUploadHandler(repo, time.Hour, func() time.Time { return now }, discardLogger())

	require.NoError(t, handler(context.Background(), nil))
	require.Equal(t, now.Add(-time.Hour), repo.cutoff)
}

func TestStaleUploadHandler_PropagatesError(t *testing.T) {
	repo := &fakeStaleRepo{err: errors.New("boom")}
	handler := StaleUploadHandler(repo, time.Hour, time.Now, discardLogger())

	require.Error(t, handler(context.Background(), nil))
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
