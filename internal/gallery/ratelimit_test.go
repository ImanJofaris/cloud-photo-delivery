package gallery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

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
