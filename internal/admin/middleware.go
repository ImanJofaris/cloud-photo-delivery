package admin

import (
	"net/http"

	"github.com/imanjofaris/cloud-photo-delivery/internal/auth"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

// RequireAdmin rejects authenticated users without the admin flag. Mount it
// after RequireAuth so the user ID is already in the request context.
func (s *Service) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			httpx.Error(w, r, apperr.Unauthorized())
			return
		}
		isAdmin, err := s.IsAdmin(r.Context(), userID)
		if err != nil {
			httpx.Error(w, r, err)
			return
		}
		if !isAdmin {
			httpx.Error(w, r, apperr.Forbidden())
			return
		}
		next.ServeHTTP(w, r)
	})
}
