package events

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/audit"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type eventDTO struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	ClientName   string  `json:"clientName"`
	ClientEmail  string  `json:"clientEmail"`
	Location     string  `json:"location"`
	Description  string  `json:"description"`
	EventDate    *string `json:"eventDate"`
	Status       string  `json:"status"`
	CoverPhotoID *string `json:"coverPhotoId"`
	StorageBytes int64   `json:"storageBytes"`
	PhotoCount   int64   `json:"photoCount"`
	GuestCount   int64   `json:"guestCount"`
	ExpiresAt    *string `json:"expiresAt"`
	CreatedAt    string  `json:"createdAt"`
	UpdatedAt    string  `json:"updatedAt"`
}

type settingsDTO struct {
	EventID               string `json:"eventId"`
	Visibility            string `json:"visibility"`
	PasswordProtected     bool   `json:"passwordProtected"`
	AllowDownload         bool   `json:"allowDownload"`
	AllowOriginalDownload bool   `json:"allowOriginalDownload"`
	WatermarkEnabled      bool   `json:"watermarkEnabled"`
	UpdatedAt             string `json:"updatedAt"`
}

type dashboardDTO struct {
	PhotoCount   int64 `json:"photoCount"`
	StorageBytes int64 `json:"storageBytes"`
	GuestCount   int64 `json:"guestCount"`
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func parseDate(s string) (*time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func toEventDTO(e *Event) eventDTO {
	dto := eventDTO{
		ID:           e.ID.String(),
		Name:         e.Name,
		Slug:         e.Slug,
		ClientName:   e.ClientName,
		ClientEmail:  e.ClientEmail,
		Location:     e.Location,
		Description:  e.Description,
		EventDate:    formatDate(e.EventDate),
		Status:       string(e.Status),
		StorageBytes: e.StorageBytes,
		PhotoCount:   e.PhotoCount,
		GuestCount:   e.GuestCount,
		ExpiresAt:    formatTimePtr(e.ExpiresAt),
		CreatedAt:    formatTime(e.CreatedAt),
		UpdatedAt:    formatTime(e.UpdatedAt),
	}
	if e.CoverPhotoID != nil {
		s := e.CoverPhotoID.String()
		dto.CoverPhotoID = &s
	}
	return dto
}

func toSettingsDTO(s *Settings) settingsDTO {
	return settingsDTO{
		EventID:               s.EventID.String(),
		Visibility:            string(s.Visibility),
		PasswordProtected:     s.PasswordHash != "",
		AllowDownload:         s.AllowDownload,
		AllowOriginalDownload: s.AllowOriginalDownload,
		WatermarkEnabled:      s.WatermarkEnabled,
		UpdatedAt:             formatTime(s.UpdatedAt),
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
	raw := chi.URLParam(r, "eventID")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, notFound()
	}
	return id, nil
}

type createRequest struct {
	Name        string  `json:"name"`
	EventDate   *string `json:"eventDate"`
	ClientName  string  `json:"clientName"`
	ClientEmail string  `json:"clientEmail"`
	Location    string  `json:"location"`
	Description string  `json:"description"`
	Status      *string `json:"status"`
	ExpiresAt   *string `json:"expiresAt"`
	Settings    *struct {
		Visibility            *string `json:"visibility"`
		Password              *string `json:"password"`
		AllowDownload         *bool   `json:"allowDownload"`
		AllowOriginalDownload *bool   `json:"allowOriginalDownload"`
		WatermarkEnabled      *bool   `json:"watermarkEnabled"`
	} `json:"settings"`
}

func decodeBody(r *http.Request, dst any) error {
	return httpx.DecodeJSONStrict(r, dst, 1<<20)
}

func toSettingsInput(in *struct {
	Visibility            *string `json:"visibility"`
	Password              *string `json:"password"`
	AllowDownload         *bool   `json:"allowDownload"`
	AllowOriginalDownload *bool   `json:"allowOriginalDownload"`
	WatermarkEnabled      *bool   `json:"watermarkEnabled"`
}) SettingsInput {
	if in == nil {
		return SettingsInput{}
	}
	out := SettingsInput{
		Password:              in.Password,
		AllowDownload:         in.AllowDownload,
		AllowOriginalDownload: in.AllowOriginalDownload,
		WatermarkEnabled:      in.WatermarkEnabled,
	}
	if in.Visibility != nil {
		v := Visibility(*in.Visibility)
		out.Visibility = &v
	}
	return out
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req createRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}

	params := CreateParams{
		Name:        req.Name,
		ClientName:  req.ClientName,
		ClientEmail: req.ClientEmail,
		Location:    req.Location,
		Description: req.Description,
		Settings:    toSettingsInput(req.Settings),
	}
	if req.Status != nil {
		params.Status = Status(*req.Status)
	}
	if req.EventDate != nil {
		d, err := parseDate(*req.EventDate)
		if err != nil {
			httpx.Error(w, r, validationError("eventDate must be YYYY-MM-DD"))
			return
		}
		params.EventDate = d
	}
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			httpx.Error(w, r, validationError("expiresAt must be RFC3339"))
			return
		}
		params.ExpiresAt = &t
	}

	event, settings, err := h.svc.Create(r.Context(), userID, params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusCreated, map[string]any{
		"event":    toEventDTO(event),
		"settings": toSettingsDTO(settings),
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	q := r.URL.Query()
	params := ListParams{
		Query:  q.Get("q"),
		Cursor: q.Get("cursor"),
		Limit:  parseLimit(q.Get("limit")),
	}
	if s := q.Get("status"); s != "" {
		st := Status(s)
		params.Status = &st
	}

	result, err := h.svc.List(r.Context(), userID, params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]eventDTO, 0, len(result.Events))
	for _, e := range result.Events {
		items = append(items, toEventDTO(e))
	}
	data := map[string]any{"items": items}
	if result.NextCursor != "" {
		data["nextCursor"] = result.NextCursor
	} else {
		data["nextCursor"] = nil
	}
	httpx.Success(w, http.StatusOK, data)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	event, err := h.svc.Get(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toEventDTO(event))
}

type updateRequest struct {
	Name        *string `json:"name"`
	EventDate   *string `json:"eventDate"`
	ClientName  *string `json:"clientName"`
	ClientEmail *string `json:"clientEmail"`
	Location    *string `json:"location"`
	Description *string `json:"description"`
	ExpiresAt   *string `json:"expiresAt"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req updateRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	params := UpdateParams{
		Name:        req.Name,
		ClientName:  req.ClientName,
		ClientEmail: req.ClientEmail,
		Location:    req.Location,
		Description: req.Description,
	}
	if req.EventDate != nil {
		if *req.EventDate == "" {
			params.ClearDate = true
		} else {
			d, err := parseDate(*req.EventDate)
			if err != nil {
				httpx.Error(w, r, validationError("eventDate must be YYYY-MM-DD"))
				return
			}
			params.EventDate = d
		}
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			params.ClearExpiry = true
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				httpx.Error(w, r, validationError("expiresAt must be RFC3339"))
				return
			}
			params.ExpiresAt = &t
		}
	}

	event, err := h.svc.Update(r.Context(), userID, id, params)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toEventDTO(event))
}

func (h *Handler) Archive(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	event, err := h.svc.Archive(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toEventDTO(event))
}

type extendRequest struct {
	Days int `json:"days"`
}

func (h *Handler) Extend(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req extendRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	event, err := h.svc.Extend(r.Context(), userID, id, req.Days)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toEventDTO(event))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.Delete(r.Context(), userID, id); err != nil {
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "event.delete", "outcome", "success",
		"actor_id", userID.String(), "event_id", id.String())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	settings, err := h.svc.Settings(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSettingsDTO(settings))
}

type settingsRequest struct {
	Visibility            *string `json:"visibility"`
	Password              *string `json:"password"`
	AllowDownload         *bool   `json:"allowDownload"`
	AllowOriginalDownload *bool   `json:"allowOriginalDownload"`
	WatermarkEnabled      *bool   `json:"watermarkEnabled"`
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req settingsRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	in := SettingsInput{
		Password:              req.Password,
		AllowDownload:         req.AllowDownload,
		AllowOriginalDownload: req.AllowOriginalDownload,
		WatermarkEnabled:      req.WatermarkEnabled,
	}
	if req.Visibility != nil {
		v := Visibility(*req.Visibility)
		in.Visibility = &v
	}
	settings, err := h.svc.UpdateSettings(r.Context(), userID, id, in)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSettingsDTO(settings))
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	id, err := h.eventID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	event, err := h.svc.Dashboard(r.Context(), userID, id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, dashboardDTO{
		PhotoCount:   event.PhotoCount,
		StorageBytes: event.StorageBytes,
		GuestCount:   event.GuestCount,
	})
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
