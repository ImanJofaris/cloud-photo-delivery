package qr

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	DefaultSize = 512
	MinSize     = 64
	MaxSize     = 2048
)

// EventLookup resolves an operator-owned event. Implementations must scope the
// lookup to userID so QRs cannot be fetched for another tenant's event.
type EventLookup interface {
	Get(ctx context.Context, userID, id uuid.UUID) (*events.Event, error)
}

type Service struct {
	events  EventLookup
	baseURL string
}

func NewService(events EventLookup, baseURL string) *Service {
	return &Service{events: events, baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/")}
}

func (s *Service) GalleryURL(slug string) string {
	return s.baseURL + "/e/" + url.PathEscape(slug)
}

func (s *Service) EventURL(ctx context.Context, userID, eventID uuid.UUID) (string, error) {
	event, err := s.events.Get(ctx, userID, eventID)
	if err != nil {
		return "", err
	}
	return s.GalleryURL(event.Slug), nil
}

func (s *Service) PNG(ctx context.Context, userID, eventID uuid.UUID, size int) ([]byte, error) {
	target, err := s.EventURL(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	encoded, err := qrcode.Encode(target, qrcode.Medium, NormalizeSize(size))
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return encoded, nil
}

func (s *Service) SVG(ctx context.Context, userID, eventID uuid.UUID) ([]byte, error) {
	target, err := s.EventURL(ctx, userID, eventID)
	if err != nil {
		return nil, err
	}
	code, err := qrcode.New(target, qrcode.Medium)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return renderSVG(code.Bitmap()), nil
}

func NormalizeSize(size int) int {
	if size <= 0 {
		return DefaultSize
	}
	if size < MinSize {
		return MinSize
	}
	if size > MaxSize {
		return MaxSize
	}
	return size
}

func renderSVG(bitmap [][]bool) []byte {
	size := len(bitmap)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%[1]d" height="%[1]d" viewBox="0 0 %[1]d %[1]d" shape-rendering="crispEdges">`, size)
	fmt.Fprintf(&b, `<rect width="%[1]d" height="%[1]d" fill="#ffffff"/>`, size)
	b.WriteString(`<path fill="#000000" d="`)
	for y, row := range bitmap {
		x := 0
		for x < len(row) {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < len(row) && row[x] {
				x++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", start, y, x-start, x-start)
		}
	}
	b.WriteString(`"/></svg>`)
	return []byte(b.String())
}
