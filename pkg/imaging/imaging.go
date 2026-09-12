// Package imaging provides pure image decode/resize/encode helpers used by the
// processing worker. It is deliberately free of storage and database concerns
// so it can be unit-tested without I/O.
package imaging

import (
	"bytes"
	"errors"
	"fmt"
	"image"

	_ "image/jpeg"
	_ "image/png"

	"github.com/gen2brain/vpx/webp"
	"golang.org/x/image/draw"
)

// ErrUnsupported indicates the bytes are not a recognised image format.
var ErrUnsupported = errors.New("unsupported image format")

// ErrTooLarge indicates the image exceeds the configured pixel guard.
var ErrTooLarge = errors.New("image exceeds maximum pixel count")

// Decoder decodes an image and reports its format. Swap for a libvips-backed
// implementation without touching the processing service.
type Decoder interface {
	Decode(data []byte) (image.Image, string, error)
}

// Encoder encodes an image to lossy WebP bytes at the given quality (0-100).
type Encoder interface {
	Encode(img image.Image, quality int) ([]byte, error)
}

// Resizer scales an image to fit within width, preserving aspect ratio.
type Resizer interface {
	Resize(img image.Image, maxEdge int) image.Image
}

// MaxPixels bounds decoded dimensions to defend against decompression bombs.
const MaxPixels = 80_000_000

type Standard struct{}

func New() Standard { return Standard{} }

func (Standard) Decode(data []byte) (image.Image, string, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return nil, "", ErrUnsupported
	}
	if int64(b.Dx())*int64(b.Dy()) > MaxPixels {
		return nil, "", ErrTooLarge
	}
	return img, format, nil
}

func (Standard) Encode(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := webp.Encode(&buf, img, webp.EncodeOptions{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}
	return buf.Bytes(), nil
}

// Resize scales img so its longest edge is at most maxEdge. Images already
// within bounds are returned unchanged (no upscaling). Aspect ratio is
// preserved and the image is resampled with ApproxBiLinear for smooth output.
func (Standard) Resize(img image.Image, maxEdge int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxEdge && h <= maxEdge {
		return img
	}
	var nw, nh int
	if w >= h {
		nw = maxEdge
		nh = h * maxEdge / w
	} else {
		nh = maxEdge
		nw = w * maxEdge / h
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
