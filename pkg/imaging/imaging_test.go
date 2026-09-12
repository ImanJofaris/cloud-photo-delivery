package imaging

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/gen2brain/vpx/webp"
	"github.com/stretchr/testify/require"
)

func solid(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 128, 255})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

func TestStandard_DecodeJPEG(t *testing.T) {
	data := encodeJPEG(t, solid(20, 10))
	got, format, err := Standard{}.Decode(data)
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	require.Equal(t, 20, got.Bounds().Dx())
	require.Equal(t, 10, got.Bounds().Dy())
}

func TestStandard_DecodePNG(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, solid(5, 5)))
	_, format, err := Standard{}.Decode(buf.Bytes())
	require.NoError(t, err)
	require.Equal(t, "png", format)
}

func TestStandard_DecodeRejectsNonImage(t *testing.T) {
	_, _, err := Standard{}.Decode([]byte("not an image at all"))
	require.ErrorIs(t, err, ErrUnsupported)
}

func TestStandard_ResizePreservesAspectRatio(t *testing.T) {
	cases := []struct {
		name       string
		w, h, edge int
		wantW      int
		wantH      int
	}{
		{"landscape downscale", 6000, 4000, 2000, 2000, 1333},
		{"portrait downscale", 4000, 6000, 2000, 1333, 2000},
		{"square downscale", 1000, 1000, 400, 400, 400},
		{"already small unchanged", 100, 80, 400, 100, 80},
		{"exact edge unchanged", 400, 400, 400, 400, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Standard{}.Resize(solid(tc.w, tc.h), tc.edge)
			b := out.Bounds()
			require.Equal(t, tc.wantW, b.Dx())
			require.Equal(t, tc.wantH, b.Dy())
		})
	}
}

func TestStandard_ResizeNoUpscale(t *testing.T) {
	img := solid(200, 100)
	out := Standard{}.Resize(img, 400)
	require.Same(t, img, out)
}

func TestStandard_EncodeProducesLossyWebP(t *testing.T) {
	data, err := Standard{}.Encode(solid(16, 16), 80)
	require.NoError(t, err)
	require.Greater(t, len(data), 0)
	require.Equal(t, "RIFF", string(data[:4]))
	require.Equal(t, "WEBP", string(data[8:12]))
	require.Equal(t, "VP8 ", string(data[12:16]), "must be lossy VP8, not lossless VP8L")

	decoded, _, err := Standard{}.Decode(data)
	require.NoError(t, err)
	require.Equal(t, 16, decoded.Bounds().Dx())
}

// A photographic (non-uniform) image should compress materially smaller as a
// lossy WebP than a lossless one. This guards the Phase 4.1 regression where
// nativewebp always wrote VP8L.
func TestStandard_LossyIsSmallerThanLossless(t *testing.T) {
	img := noisy(512, 512)

	var lossless bytes.Buffer
	require.NoError(t, webp.Encode(&lossless, img, webp.EncodeOptions{Lossless: true}))

	lossy, err := Standard{}.Encode(img, 80)
	require.NoError(t, err)

	require.Less(t, len(lossy), lossless.Len(),
		"lossy webp (%d bytes) should be smaller than lossless (%d bytes)", len(lossy), lossless.Len())
}

func TestStandard_QualityMonotonic(t *testing.T) {
	img := noisy(512, 512)
	low, err := Standard{}.Encode(img, 30)
	require.NoError(t, err)
	high, err := Standard{}.Encode(img, 95)
	require.NoError(t, err)
	require.Less(t, len(low), len(high), "lower quality should produce a smaller file")
}

// noisy builds a high-frequency image so lossy and lossless differ clearly.
func noisy(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var seed uint32 = 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			img.Set(x, y, color.RGBA{uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 255})
		}
	}
	return img
}
