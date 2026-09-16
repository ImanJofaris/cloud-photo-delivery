package devices

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/audit"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type deviceDTO struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	KeyPrefix       string  `json:"keyPrefix"`
	AssignedEventID *string `json:"assignedEventId"`
	RevokedAt       *string `json:"revokedAt"`
	LastUsedAt      *string `json:"lastUsedAt"`
	CreatedAt       string  `json:"createdAt"`
}

type deviceWithKeyDTO struct {
	Device deviceDTO `json:"device"`
	Key    string    `json:"key"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

func toDeviceDTO(d *Device) deviceDTO {
	var assigned *string
	if d.AssignedEventID != nil {
		s := d.AssignedEventID.String()
		assigned = &s
	}
	return deviceDTO{
		ID:              d.ID.String(),
		Name:            d.Name,
		KeyPrefix:       d.KeyPrefix,
		AssignedEventID: assigned,
		RevokedAt:       formatTimePtr(d.RevokedAt),
		LastUsedAt:      formatTimePtr(d.LastUsedAt),
		CreatedAt:       formatTime(d.CreatedAt),
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

func (h *Handler) deviceID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "deviceID"))
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func decodeBody(r *http.Request, dst any) error {
	return httpx.DecodeJSONStrict(r, dst, 1<<20)
}

type createDeviceRequest struct {
	Name            string  `json:"name"`
	AssignedEventID *string `json:"assignedEventId"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req createDeviceRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	var assigned *uuid.UUID
	if req.AssignedEventID != nil && *req.AssignedEventID != "" {
		id, err := uuid.Parse(*req.AssignedEventID)
		if err != nil {
			httpx.Error(w, r, validationError("Assigned event id is invalid"))
			return
		}
		assigned = &id
	}

	device, raw, err := h.svc.Create(r.Context(), userID, CreateParams{Name: req.Name, AssignedEventID: assigned})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "device.create", "outcome", "success",
		"actor_id", userID.String(), "device_id", device.ID.String())
	httpx.Success(w, http.StatusCreated, deviceWithKeyDTO{Device: toDeviceDTO(device), Key: raw})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	out := make([]deviceDTO, 0, len(items))
	for _, d := range items {
		out = append(out, toDeviceDTO(d))
	}
	httpx.Success(w, http.StatusOK, map[string]any{"items": out})
}

type updateDeviceRequest struct {
	Name string `json:"name"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.deviceID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateDeviceRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	device, err := h.svc.Rename(r.Context(), userID, id, req.Name)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toDeviceDTO(device))
}

func (h *Handler) Rotate(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.deviceID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	device, raw, err := h.svc.Rotate(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "device.rotate", "outcome", "success",
		"actor_id", userID.String(), "device_id", id.String())
	httpx.Success(w, http.StatusOK, deviceWithKeyDTO{Device: toDeviceDTO(device), Key: raw})
}

func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.deviceID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Revoke(r.Context(), userID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "device.revoke", "outcome", "success",
		"actor_id", userID.String(), "device_id", id.String())
	w.WriteHeader(http.StatusNoContent)
}
