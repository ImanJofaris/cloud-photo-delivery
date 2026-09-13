package analytics

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type countersDTO struct {
	GalleryViews   int64 `json:"galleryViews"`
	UniqueVisitors int64 `json:"uniqueVisitors"`
	Downloads      int64 `json:"downloads"`
	QRScans        int64 `json:"qrScans"`
}

type dayDTO struct {
	Date string `json:"date"`
	countersDTO
}

type eventAnalyticsDTO struct {
	EventID    string      `json:"eventId"`
	PhotoCount int64       `json:"photoCount"`
	Totals     countersDTO `json:"totals"`
	Daily      []dayDTO    `json:"daily"`
}

type accountAnalyticsDTO struct {
	EventCount int64       `json:"eventCount"`
	PhotoCount int64       `json:"photoCount"`
	Totals     countersDTO `json:"totals"`
	Daily      []dayDTO    `json:"daily"`
}

func toCountersDTO(c Counters) countersDTO {
	return countersDTO{
		GalleryViews:   c.GalleryViews,
		UniqueVisitors: c.UniqueVisitors,
		Downloads:      c.Downloads,
		QRScans:        c.QRScans,
	}
}

func toDayDTOs(days []DayCounters) []dayDTO {
	out := make([]dayDTO, 0, len(days))
	for _, d := range days {
		out = append(out, dayDTO{Date: d.Day.Format("2006-01-02"), countersDTO: toCountersDTO(d.Counters)})
	}
	return out
}

func toEventDTO(r *EventReport) eventAnalyticsDTO {
	return eventAnalyticsDTO{
		EventID:    r.EventID.String(),
		PhotoCount: r.PhotoCount,
		Totals:     toCountersDTO(r.Totals),
		Daily:      toDayDTOs(r.Daily),
	}
}

func toAccountDTO(r *AccountReport) accountAnalyticsDTO {
	return accountAnalyticsDTO{
		EventCount: r.EventCount,
		PhotoCount: r.PhotoCount,
		Totals:     toCountersDTO(r.Totals),
		Daily:      toDayDTOs(r.Daily),
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

func parseDays(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxDays {
		return 0, validationError("days must be between 1 and 365")
	}
	return n, nil
}

func (h *Handler) Event(w http.ResponseWriter, r *http.Request) {
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
	days, err := parseDays(r.URL.Query().Get("days"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	report, err := h.svc.Event(r.Context(), userID, eventID, days)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toEventDTO(report))
}

func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	days, err := parseDays(r.URL.Query().Get("days"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	report, err := h.svc.Account(r.Context(), userID, days)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toAccountDTO(report))
}
