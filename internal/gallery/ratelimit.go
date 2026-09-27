package gallery

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

const maxCostBodyBytes = 1 << 20

// SignedURLRateLimitKey scopes the signing budget to one client IP per event.
// Guests behind the same NAT share a bucket, but unrelated events do not, so
// one busy venue cannot exhaust another event's budget.
func SignedURLRateLimitKey(r *http.Request) string {
	ip := httpx.ClientIP(r)
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		return ip
	}
	return ip + "|" + slug
}

// BatchURLCost charges one token per requested photo ID. The body is restored
// for the handler; malformed bodies cost a single token and are rejected with
// a validation error by the handler itself.
func BatchURLCost(r *http.Request) int {
	if r.Body == nil {
		return 1
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxCostBodyBytes+1))
	if err != nil {
		return 1
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if len(body) > maxCostBodyBytes {
		return 1
	}
	var req batchPhotoURLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return 1
	}
	switch {
	case len(req.PhotoIDs) < 1:
		return 1
	case len(req.PhotoIDs) > MaxPhotoURLBatch:
		return MaxPhotoURLBatch
	default:
		return len(req.PhotoIDs)
	}
}
