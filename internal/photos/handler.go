package photos

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type photoDTO struct {
	ID           string   `json:"id"`
	Filename     string   `json:"filename"`
	MimeType     string   `json:"mimeType"`
	FileSize     int64    `json:"fileSize"`
	Width        *int     `json:"width"`
	Height       *int     `json:"height"`
	Status       string   `json:"status"`
	Variants     []string `json:"variants"`
	ErrorMessage *string  `json:"errorMessage"`
	CreatedAt    string   `json:"createdAt"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toPhotoDTO(p *Photo) photoDTO {
	filename := ""
	if p.OriginalFilename != nil {
		filename = *p.OriginalFilename
	}
	return photoDTO{
		ID:           p.ID.String(),
		Filename:     filename,
		MimeType:     p.MimeType,
		FileSize:     p.FileSize,
		Width:        p.Width,
		Height:       p.Height,
		Status:       string(p.Status),
		Variants:     VariantNames(p),
		ErrorMessage: p.ErrorMessage,
		CreatedAt:    formatTime(p.CreatedAt),
	}
}

type Handler struct {
	svc     *Service
	resolve func(*http.Request) (Actor, bool)
}

func NewHandler(svc *Service, resolveActor func(*http.Request) (Actor, bool)) *Handler {
	return &Handler{svc: svc, resolve: resolveActor}
}

func (h *Handler) actor(r *http.Request) (Actor, error) {
	a, ok := h.resolve(r)
	if !ok || a.UserID == uuid.Nil {
		return Actor{}, apperr.Unauthorized()
	}
	return a, nil
}

func parseUUIDParam(r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil || id == uuid.Nil {
		return uuid.Nil, false
	}
	return id, true
}

func parseLimit(raw string) int {
	if raw == "" {
		return 0
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 10000 {
			return 10000
		}
	}
	return n
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	eventID, ok := parseUUIDParam(r, "eventID")
	if !ok {
		httpx.Error(w, r, eventNotFound())
		return
	}

	q := r.URL.Query()
	result, err := h.svc.List(r.Context(), actor, eventID, q.Get("cursor"), q.Get("status"), parseLimit(q.Get("limit")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	items := make([]photoDTO, 0, len(result.Items))
	for _, p := range result.Items {
		items = append(items, toPhotoDTO(p))
	}
	var next any
	if result.NextCursor != "" {
		next = result.NextCursor
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"items":      items,
		"nextCursor": next,
	})
}

func (h *Handler) URL(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, ok := parseUUIDParam(r, "photoID")
	if !ok {
		httpx.Error(w, r, notFound())
		return
	}
	result, err := h.svc.URL(r.Context(), actor, photoID, r.URL.Query().Get("variant"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	httpx.Success(w, http.StatusOK, map[string]any{
		"url":       result.URL,
		"expiresIn": result.ExpiresIn,
	})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, ok := parseUUIDParam(r, "photoID")
	if !ok {
		httpx.Error(w, r, notFound())
		return
	}
	if err := h.svc.Delete(r.Context(), actor, photoID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
