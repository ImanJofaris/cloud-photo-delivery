package qr

import (
	"bytes"
	"context"
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "update golden files")

type fakeEventLookup struct {
	userID uuid.UUID
	event  *events.Event
	err    error
}

func (f *fakeEventLookup) Get(_ context.Context, userID, id uuid.UUID) (*events.Event, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.event == nil || f.event.ID != id || userID != f.userID {
		return nil, apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
	}
	return f.event, nil
}

func newTestService() (*Service, *fakeEventLookup) {
	event := &events.Event{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Name:   "Iman Wedding",
		Slug:   "iman-wedding",
	}
	lookup := &fakeEventLookup{userID: event.UserID, event: event}
	return NewService(lookup, "https://photos.example.com/"), lookup
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func decodePNG(t *testing.T, data []byte) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	require.NoError(t, err)
	result, err := gozxingqr.NewQRCodeReader().Decode(bitmap, nil)
	require.NoError(t, err)
	return result.GetText()
}

func TestService_EventURL_Canonical(t *testing.T) {
	svc, lookup := newTestService()
	got, err := svc.EventURL(context.Background(), lookup.userID, lookup.event.ID)
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.com/e/iman-wedding", got)
}

func TestService_GalleryURL_TrimsBase(t *testing.T) {
	svc := NewService(&fakeEventLookup{}, "  http://localhost:3000/  ")
	require.Equal(t, "http://localhost:3000/e/x", svc.GalleryURL("x"))
}

func TestService_EventURL_OtherTenant(t *testing.T) {
	svc, lookup := newTestService()
	_, err := svc.EventURL(context.Background(), uuid.New(), lookup.event.ID)
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_PNG_DecodesBackToURL(t *testing.T) {
	svc, lookup := newTestService()
	data, err := svc.PNG(context.Background(), lookup.userID, lookup.event.ID, 512)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	require.Equal(t, "https://photos.example.com/e/iman-wedding", decodePNG(t, data))
}

func TestService_PNG_SizeClamped(t *testing.T) {
	svc, lookup := newTestService()

	small, err := svc.PNG(context.Background(), lookup.userID, lookup.event.ID, 1)
	require.NoError(t, err)
	smallImg, err := png.Decode(bytes.NewReader(small))
	require.NoError(t, err)
	require.GreaterOrEqual(t, smallImg.Bounds().Dx(), MinSize)

	large, err := svc.PNG(context.Background(), lookup.userID, lookup.event.ID, 100000)
	require.NoError(t, err)
	largeImg, err := png.Decode(bytes.NewReader(large))
	require.NoError(t, err)
	require.Equal(t, MaxSize, largeImg.Bounds().Dx())

	deflt, err := svc.PNG(context.Background(), lookup.userID, lookup.event.ID, 0)
	require.NoError(t, err)
	defltImg, err := png.Decode(bytes.NewReader(deflt))
	require.NoError(t, err)
	require.Equal(t, DefaultSize, defltImg.Bounds().Dx())
}

func TestService_SVG_MatchesGolden(t *testing.T) {
	svc, lookup := newTestService()
	got, err := svc.SVG(context.Background(), lookup.userID, lookup.event.ID)
	require.NoError(t, err)
	require.Contains(t, string(got), `xmlns="http://www.w3.org/2000/svg"`)

	golden := filepath.Join("testdata", "event_qr.svg")
	if *update {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, got, 0o644))
	}
	want, err := os.ReadFile(golden)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

func TestNormalizeSize(t *testing.T) {
	require.Equal(t, DefaultSize, NormalizeSize(0))
	require.Equal(t, DefaultSize, NormalizeSize(-5))
	require.Equal(t, MinSize, NormalizeSize(10))
	require.Equal(t, 300, NormalizeSize(300))
	require.Equal(t, MaxSize, NormalizeSize(5000))
}
