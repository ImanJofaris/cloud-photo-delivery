package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

// DecodeJSON decodes a JSON request body capped at maxBytes. An oversized
// body maps to PAYLOAD_TOO_LARGE (413) rather than a generic validation error.
func DecodeJSON(r *http.Request, dst any, maxBytes int64) error {
	return decodeJSON(r, dst, maxBytes, false)
}

// DecodeJSONStrict behaves like DecodeJSON but rejects unknown fields.
func DecodeJSONStrict(r *http.Request, dst any, maxBytes int64) error {
	return decodeJSON(r, dst, maxBytes, true)
}

func decodeJSON(r *http.Request, dst any, maxBytes int64, strict bool) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBytes))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apperr.New("PAYLOAD_TOO_LARGE", "Request body is too large", http.StatusRequestEntityTooLarge)
		}
		return apperr.New("VALIDATION_ERROR", "Request body is not valid JSON", http.StatusUnprocessableEntity)
	}
	return nil
}
