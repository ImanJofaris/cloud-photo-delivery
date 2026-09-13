package qr

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

const (
	PNGContentType = "image/png"
	SVGContentType = "image/svg+xml"

	cacheQR  = "public, max-age=3600"
	cacheURL = "private, max-age=60"
)

func notFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

type Handler struct {
	svc   *Service
	getID func(*http.Request) (string, bool)
}

func NewHandler(svc *Service, resolveUserID func(*http.Request) (string, bool)) *Handler {
	return &Handler{svc: svc, getID: resolveUserID}
}

func (h *Handler) userID(r *http.Request) (uuid.UUID, error) {
	raw, ok := h.getID(r)
	if !ok {
		return uuid.Nil, apperr.Unauthorized()
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, apperr.Unauthorized().WithCause(err)
	}
	return id, nil
}

func (h *Handler) eventID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func (h *Handler) URL(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	eventID, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	target, err := h.svc.EventURL(r.Context(), userID, eventID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", cacheURL)
	httpx.Success(w, http.StatusOK, map[string]string{"url": target})
}

func (h *Handler) PNG(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	eventID, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	encoded, err := h.svc.PNG(r.Context(), userID, eventID, parseSize(r.URL.Query().Get("size")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", PNGContentType)
	w.Header().Set("Cache-Control", cacheQR)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func (h *Handler) SVG(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	eventID, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	encoded, err := h.svc.SVG(r.Context(), userID, eventID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Content-Type", SVGContentType)
	w.Header().Set("Cache-Control", cacheQR)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

func parseSize(raw string) int {
	if raw == "" {
		return 0
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 100000 {
			return 100000
		}
	}
	return n
}
