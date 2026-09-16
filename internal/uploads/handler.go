package uploads

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type uploadStatusDTO struct {
	PhotoID      string  `json:"photoId"`
	EventID      string  `json:"eventId"`
	Status       string  `json:"status"`
	UploadKind   string  `json:"uploadKind"`
	Filename     string  `json:"filename"`
	MimeType     string  `json:"mimeType"`
	FileSize     int64   `json:"fileSize"`
	ErrorMessage *string `json:"errorMessage"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func toUploadStatusDTO(p *photos.Photo) uploadStatusDTO {
	filename := ""
	if p.OriginalFilename != nil {
		filename = *p.OriginalFilename
	}
	return uploadStatusDTO{
		PhotoID:      p.ID.String(),
		EventID:      p.EventID.String(),
		Status:       string(p.Status),
		UploadKind:   string(p.UploadKind),
		Filename:     filename,
		MimeType:     p.MimeType,
		FileSize:     p.FileSize,
		ErrorMessage: p.ErrorMessage,
		CreatedAt:    formatTime(p.CreatedAt),
		UpdatedAt:    formatTime(p.UpdatedAt),
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

func (h *Handler) eventID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "eventID"))
	if err != nil {
		return uuid.Nil, apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
	}
	return id, nil
}

func (h *Handler) photoID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "photoID"))
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

func decodeBody(r *http.Request, dst any) error {
	return httpx.DecodeJSONStrict(r, dst, 1<<20)
}

type initializeRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func (h *Handler) Initialize(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	eventID, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req initializeRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	result, err := h.svc.Initialize(r.Context(), actor, InitParams{
		EventID:        eventID,
		Filename:       req.Filename,
		ContentType:    req.ContentType,
		Size:           req.Size,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}

	data := map[string]any{
		"photoId":    result.PhotoID.String(),
		"uploadKind": string(result.UploadKind),
		"storageKey": result.StorageKey,
		"expiresAt":  formatTime(result.ExpiresAt),
	}
	if result.UploadKind == photos.KindMultipart {
		data["uploadUrl"] = result.UploadURL
		data["partSize"] = result.PartSize
	} else {
		data["uploadUrl"] = result.UploadURL
		data["partSize"] = nil
	}
	httpx.Success(w, http.StatusCreated, data)
}

func (h *Handler) RePresign(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	result, err := h.svc.RePresign(r.Context(), actor, photoID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"uploadUrl": result.UploadURL,
		"expiresAt": formatTime(result.ExpiresAt),
	})
}

type partsRequest struct {
	PartNumbers []int `json:"partNumbers"`
}

func (h *Handler) Parts(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req partsRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	result, err := h.svc.Parts(r.Context(), actor, photoID, req.PartNumbers)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	parts := make([]map[string]any, 0, len(result.Parts))
	for _, p := range result.Parts {
		parts = append(parts, map[string]any{"partNumber": p.PartNumber, "url": p.URL})
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"photoId":  result.PhotoID.String(),
		"partSize": result.PartSize,
		"parts":    parts,
	})
}

type completeMultipartRequest struct {
	Parts []struct {
		PartNumber int    `json:"partNumber"`
		ETag       string `json:"etag"`
	} `json:"parts"`
}

func (h *Handler) CompleteMultipart(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req completeMultipartRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	parts := make([]CompletedPart, 0, len(req.Parts))
	for _, p := range req.Parts {
		parts = append(parts, CompletedPart{PartNumber: p.PartNumber, ETag: p.ETag})
	}

	if err := h.svc.CompleteMultipart(r.Context(), actor, photoID, parts); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]any{"photoId": photoID.String()})
}

func (h *Handler) AbortMultipart(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.AbortMultipart(r.Context(), actor, photoID); err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photo, err := h.svc.Complete(r.Context(), actor, photoID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toUploadStatusDTO(photo))
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photoID, err := h.photoID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	photo, err := h.svc.Status(r.Context(), actor, photoID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toUploadStatusDTO(photo))
}
