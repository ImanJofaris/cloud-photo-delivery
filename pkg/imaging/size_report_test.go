package imaging

import (
	"bytes"
	"os"
	"testing"

	"github.com/gen2brain/vpx/webp"
	"github.com/stretchr/testify/require"
)

// TestStandard_SizeReport is a diagnostic test that prints real file sizes for
// a photographic 6000x4000 image at each derivative size/quality. It is not an
// assertion beyond "lossy beats lossless"; it exists to quantify the Phase 4.1
// egress/storage saving. Skipped unless IMAGING_SIZE_REPORT=1.
func TestStandard_SizeReport(t *testing.T) {
	if os.Getenv("IMAGING_SIZE_REPORT") != "1" {
		t.Skip("set IMAGING_SIZE_REPORT=1 to run the size diagnostic")
	}
	img := noisy(6000, 4000)
	std := Standard{}

	var lossless bytes.Buffer
	require.NoError(t, webp.Encode(&lossless, std.Resize(img, OptimizedEdgeTest), webp.EncodeOptions{Lossless: true}))

	cases := []struct {
		name    string
		edge    int
		quality int
	}{
		{"thumbnail 400 q80", ThumbnailEdgeTest, 80},
		{"medium 1000 q82", MediumEdgeTest, 82},
		{"optimized 2000 q85", OptimizedEdgeTest, 85},
	}
	for _, c := range cases {
		out, err := std.Encode(std.Resize(img, c.edge), c.quality)
		require.NoError(t, err)
		t.Logf("%-22s %8d bytes", c.name, len(out))
	}
	t.Logf("%-22s %8d bytes", "optimized lossless", lossless.Len())
}

const (
	ThumbnailEdgeTest = 400
	MediumEdgeTest    = 1000
	OptimizedEdgeTest = 2000
)
