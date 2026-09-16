package users

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

type profileDTO struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	BusinessName string `json:"businessName"`
}

func toProfileDTO(u *User) profileDTO {
	return profileDTO{ID: u.ID.String(), Email: u.Email, BusinessName: u.BusinessName}
}

type Handler struct {
	svc   *Service
	getID func(*http.Request) (string, bool)
}

// NewHandler requires a resolver that extracts the authenticated user ID from
// the request context (supplied by the auth middleware).
func NewHandler(svc *Service, resolveUserID func(*http.Request) (string, bool)) *Handler {
	return &Handler{svc: svc, getID: resolveUserID}
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	rawID, ok := h.getID(r)
	if !ok {
		httpx.Error(w, r, apperr.Unauthorized())
		return
	}
	id, err := parseUUID(rawID)
	if err != nil {
		httpx.Error(w, r, apperr.Unauthorized().WithCause(err))
		return
	}
	u, err := h.svc.GetProfile(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toProfileDTO(u))
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	rawID, ok := h.getID(r)
	if !ok {
		httpx.Error(w, r, apperr.Unauthorized())
		return
	}
	id, err := parseUUID(rawID)
	if err != nil {
		httpx.Error(w, r, apperr.Unauthorized().WithCause(err))
		return
	}

	var in struct {
		BusinessName *string `json:"businessName"`
	}
	if err := httpx.DecodeJSON(r, &in, 1<<20); err != nil {
		httpx.Error(w, r, err)
		return
	}

	u, err := h.svc.GetProfile(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	name := u.BusinessName
	if in.BusinessName != nil {
		name = *in.BusinessName
	}
	updated, err := h.svc.UpdateBusinessName(r.Context(), id, name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toProfileDTO(updated))
}
