package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type statsDTO struct {
	Users         int64 `json:"users"`
	Events        int64 `json:"events"`
	Photos        int64 `json:"photos"`
	StorageBytes  int64 `json:"storageBytes"`
	RevenueCents  int64 `json:"revenueCents"`
	Subscriptions int64 `json:"subscriptions"`
}

type userDTO struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	BusinessName string    `json:"businessName"`
	IsAdmin      bool      `json:"isAdmin"`
	StorageBytes int64     `json:"storageBytes"`
	EventCount   int64     `json:"eventCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

type subscriptionDTO struct {
	ID                string     `json:"id"`
	UserID            string     `json:"userId"`
	UserEmail         string     `json:"userEmail"`
	PlanID            string     `json:"planId"`
	Status            string     `json:"status"`
	Interval          string     `json:"interval"`
	CurrentPeriodEnd  *time.Time `json:"currentPeriodEnd"`
	CancelAtPeriodEnd bool       `json:"cancelAtPeriodEnd"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type queueHealthDTO struct {
	Pending         int64      `json:"pending"`
	Running         int64      `json:"running"`
	Failed          int64      `json:"failed"`
	OldestPendingAt *time.Time `json:"oldestPendingAt"`
}

type healthDTO struct {
	Status     string         `json:"status"`
	QueueDepth int64          `json:"queueDepth"`
	Queue      queueHealthDTO `json:"queue"`
	CheckedAt  time.Time      `json:"checkedAt"`
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, statsDTO{
		Users:         stats.Users,
		Events:        stats.Events,
		Photos:        stats.Photos,
		StorageBytes:  stats.StorageBytes,
		RevenueCents:  stats.RevenueCents,
		Subscriptions: stats.Subscriptions,
	})
}

func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.svc.ListUsers(r.Context(), q.Get("cursor"), parseLimit(q.Get("limit")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	users := make([]userDTO, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, userDTO{
			ID:           u.ID.String(),
			Email:        u.Email,
			BusinessName: u.BusinessName,
			IsAdmin:      u.IsAdmin,
			StorageBytes: u.StorageBytes,
			EventCount:   u.EventCount,
			CreatedAt:    u.CreatedAt,
		})
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"users":      users,
		"nextCursor": cursorOrNil(result.NextCursor),
	})
}

func (h *Handler) Subscriptions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	result, err := h.svc.ListSubscriptions(r.Context(), q.Get("cursor"), parseLimit(q.Get("limit")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	subs := make([]subscriptionDTO, 0, len(result.Subscriptions))
	for _, s := range result.Subscriptions {
		subs = append(subs, subscriptionDTO{
			ID:                s.ID.String(),
			UserID:            s.UserID.String(),
			UserEmail:         s.UserEmail,
			PlanID:            s.PlanID,
			Status:            s.Status,
			Interval:          s.Interval,
			CurrentPeriodEnd:  s.CurrentPeriodEnd,
			CancelAtPeriodEnd: s.CancelAtPeriodEnd,
			CreatedAt:         s.CreatedAt,
		})
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"subscriptions": subs,
		"nextCursor":    cursorOrNil(result.NextCursor),
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	health, err := h.svc.Health(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, healthDTO{
		Status:     health.Status,
		QueueDepth: health.QueueDepth(),
		Queue: queueHealthDTO{
			Pending:         health.Queue.Pending,
			Running:         health.Queue.Running,
			Failed:          health.Queue.Failed,
			OldestPendingAt: health.Queue.OldestPendingAt,
		},
		CheckedAt: health.CheckedAt,
	})
}

func parseLimit(raw string) int {
	if raw == "" {
		return 0
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return n
}

func cursorOrNil(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}
