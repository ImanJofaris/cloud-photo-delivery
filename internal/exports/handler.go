package exports

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type exportDTO struct {
	ID          string  `json:"id"`
	EventID     string  `json:"eventId"`
	Status      string  `json:"status"`
	FileSize    *int64  `json:"fileSize"`
	ExpiresAt   *string `json:"expiresAt"`
	CreatedAt   string  `json:"createdAt"`
	DownloadURL *string `json:"downloadUrl"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

func toExportDTO(v *View) exportDTO {
	return exportDTO{
		ID:          v.Export.ID.String(),
		EventID:     v.Export.EventID.String(),
		Status:      string(v.Export.Status),
		FileSize:    v.Export.FileSize,
		ExpiresAt:   formatTimePtr(v.Export.ExpiresAt),
		CreatedAt:   formatTime(v.Export.CreatedAt),
		DownloadURL: v.DownloadURL,
	}
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
		return uuid.Nil, eventNotFound()
	}
	return id, nil
}

func (h *Handler) exportID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "exportID"))
	if err != nil {
		return uuid.Nil, exportNotFound()
	}
	return id, nil
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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
	view, err := h.svc.Create(r.Context(), userID, eventID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusAccepted, toExportDTO(view))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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
	exportID, err := h.exportID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.Get(r.Context(), userID, eventID, exportID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toExportDTO(view))
}
