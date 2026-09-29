package gallery

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchURLCost_CountsPhotoIDs(t *testing.T) {
	body := `{"variant":"thumbnail","photoIds":["a","b","c"]}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	require.Equal(t, 3, BatchURLCost(r))

	restored, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.Equal(t, body, string(restored), "the handler must still see the body")
}

func TestBatchURLCost_MalformedOrEmptyCostsOne(t *testing.T) {
	for _, body := range []string{`{bad`, `{"variant":"thumbnail"}`, `{"photoIds":[]}`} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		require.Equal(t, 1, BatchURLCost(r), "body=%s", body)
	}
}

func TestBatchURLCost_CapsAtMaxBatch(t *testing.T) {
	ids := make([]string, 0, MaxPhotoURLBatch+50)
	for i := 0; i < MaxPhotoURLBatch+50; i++ {
		ids = append(ids, `"a"`)
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"photoIds":[`+strings.Join(ids, ",")+`]}`))
	require.Equal(t, MaxPhotoURLBatch, BatchURLCost(r))
}

func TestSignedURLRateLimitKey_ScopesToEvent(t *testing.T) {
	key := SignedURLRateLimitKey(func(r *http.Request) string {
		return r.Header.Get("X-Forwarded-For")
	})
	makeReq := func(slug, ip string) *http.Request {
		r := withParams(httptest.NewRequest(http.MethodGet, "/", nil), map[string]string{"slug": slug})
		r.Header.Set("X-Forwarded-For", ip)
		return r
	}

	eventA := key(makeReq("wedding", "1.2.3.4"))
	eventB := key(makeReq("party", "1.2.3.4"))
	otherIP := key(makeReq("wedding", "5.6.7.8"))

	require.NotEqual(t, eventA, eventB, "events must not share a bucket")
	require.NotEqual(t, eventA, otherIP, "IPs must not share a bucket")
	require.Equal(t, eventA, key(makeReq("wedding", "1.2.3.4")))
}
