package billing

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/billing/provider"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type planDTO struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	PriceCents int           `json:"priceCents"`
	Currency   string        `json:"currency"`
	Interval   string        `json:"interval"`
	Limits     planLimitsDTO `json:"limits"`
	Active     bool          `json:"active"`
}

type planLimitsDTO struct {
	Events            int   `json:"events"`
	PhotosPerEvent    int   `json:"photosPerEvent"`
	StorageBytes      int64 `json:"storageBytes"`
	RetentionDays     int   `json:"retentionDays"`
	APIAccess         bool  `json:"apiAccess"`
	Branding          bool  `json:"branding"`
	OriginalDownloads bool  `json:"originalDownloads"`
}

type subscriptionDTO struct {
	ID                 string  `json:"id"`
	PlanID             string  `json:"planId"`
	Status             string  `json:"status"`
	Provider           *string `json:"provider"`
	ProviderRef        *string `json:"providerRef"`
	Interval           string  `json:"interval"`
	CurrentPeriodStart *string `json:"currentPeriodStart"`
	CurrentPeriodEnd   *string `json:"currentPeriodEnd"`
	CancelAtPeriodEnd  bool    `json:"cancelAtPeriodEnd"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
}

type usageDTO struct {
	Events       int   `json:"events"`
	StorageBytes int64 `json:"storageBytes"`
}

type invoiceDTO struct {
	ID             string  `json:"id"`
	SubscriptionID *string `json:"subscriptionId"`
	AmountCents    int     `json:"amountCents"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	ProviderRef    *string `json:"providerRef"`
	IssuedAt       string  `json:"issuedAt"`
	PaidAt         *string `json:"paidAt"`
}

type checkoutDTO struct {
	ProviderRef string `json:"providerRef"`
	URL         string `json:"url"`
	ExpiresAt   string `json:"expiresAt"`
}

type Handler struct {
	svc      *Service
	provider provider.PaymentProvider
	getID    func(*http.Request) (string, bool)
}

func NewHandler(svc *Service, p provider.PaymentProvider, resolveUserID func(*http.Request) (string, bool)) *Handler {
	return &Handler{svc: svc, provider: p, getID: resolveUserID}
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := formatTime(*t)
	return &s
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toPlanDTO(p *Plan) planDTO {
	return planDTO{
		ID:         p.ID,
		Name:       p.Name,
		PriceCents: p.PriceCents,
		Currency:   p.Currency,
		Interval:   p.Interval,
		Limits: planLimitsDTO{
			Events:            p.Limits.Events,
			PhotosPerEvent:    p.Limits.PhotosPerEvent,
			StorageBytes:      p.Limits.StorageBytes,
			RetentionDays:     p.Limits.RetentionDays,
			APIAccess:         p.Limits.APIAccess,
			Branding:          p.Limits.Branding,
			OriginalDownloads: p.Limits.OriginalDownloads,
		},
		Active: p.Active,
	}
}

func toSubscriptionDTO(s *Subscription) subscriptionDTO {
	dto := subscriptionDTO{
		ID:                 s.ID.String(),
		PlanID:             s.PlanID,
		Status:             string(s.Status),
		Provider:           nullIfEmpty(s.Provider),
		ProviderRef:        s.ProviderRef,
		Interval:           s.Interval,
		CurrentPeriodStart: formatTimePtr(s.CurrentPeriodStart),
		CurrentPeriodEnd:   formatTimePtr(s.CurrentPeriodEnd),
		CancelAtPeriodEnd:  s.CancelAtPeriodEnd,
		CreatedAt:          formatTime(s.CreatedAt),
		UpdatedAt:          formatTime(s.UpdatedAt),
	}
	return dto
}

func toInvoiceDTO(inv *Invoice) invoiceDTO {
	dto := invoiceDTO{
		ID:          inv.ID.String(),
		AmountCents: inv.AmountCents,
		Currency:    inv.Currency,
		Status:      string(inv.Status),
		ProviderRef: inv.ProviderRef,
		IssuedAt:    formatTime(inv.IssuedAt),
		PaidAt:      formatTimePtr(inv.PaidAt),
	}
	if inv.SubscriptionID != nil {
		s := inv.SubscriptionID.String()
		dto.SubscriptionID = &s
	}
	return dto
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

func decodeBody(r *http.Request, dst any) error {
	return httpx.DecodeJSONStrict(r, dst, 1<<20)
}

func (h *Handler) Plans(w http.ResponseWriter, r *http.Request) {
	if _, err := h.userID(r); err != nil {
		httpx.Error(w, r, err)
		return
	}
	plans, err := h.svc.Plans(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]planDTO, 0, len(plans))
	for _, p := range plans {
		items = append(items, toPlanDTO(p))
	}
	httpx.Success(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.GetSubscription(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var sub any
	if view.Subscription != nil {
		sub = toSubscriptionDTO(view.Subscription)
	}
	httpx.Success(w, http.StatusOK, map[string]any{
		"subscription": sub,
		"plan":         toPlanDTO(view.Plan),
		"usage": usageDTO{
			Events:       view.Usage.Events,
			StorageBytes: view.Usage.StorageBytes,
		},
	})
}

type subscribeRequest struct {
	PlanID   string `json:"planId"`
	Interval string `json:"interval"`
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req subscribeRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	sub, checkout, err := h.svc.Subscribe(r.Context(), userID, req.PlanID, req.Interval)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusCreated, map[string]any{
		"subscription": toSubscriptionDTO(sub),
		"checkout": checkoutDTO{
			ProviderRef: checkout.ProviderRef,
			URL:         checkout.URL,
			ExpiresAt:   formatTime(checkout.ExpiresAt),
		},
	})
}

type planChangeRequest struct {
	PlanID string `json:"planId"`
}

func (h *Handler) Upgrade(w http.ResponseWriter, r *http.Request) {
	h.changePlan(w, r, true)
}

func (h *Handler) Downgrade(w http.ResponseWriter, r *http.Request) {
	h.changePlan(w, r, false)
}

func (h *Handler) changePlan(w http.ResponseWriter, r *http.Request, up bool) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req planChangeRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	var sub *Subscription
	if up {
		sub, err = h.svc.Upgrade(r.Context(), userID, req.PlanID)
	} else {
		sub, err = h.svc.Downgrade(r.Context(), userID, req.PlanID)
	}
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSubscriptionDTO(sub))
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	sub, err := h.svc.Cancel(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSubscriptionDTO(sub))
}

func (h *Handler) Resume(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	sub, err := h.svc.Resume(r.Context(), userID)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toSubscriptionDTO(sub))
}

func (h *Handler) Invoices(w http.ResponseWriter, r *http.Request) {
	userID, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	q := r.URL.Query()
	result, err := h.svc.Invoices(r.Context(), userID, q.Get("cursor"), parseLimit(q.Get("limit")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]invoiceDTO, 0, len(result.Invoices))
	for _, inv := range result.Invoices {
		items = append(items, toInvoiceDTO(inv))
	}
	data := map[string]any{"items": items}
	if result.NextCursor != "" {
		data["nextCursor"] = result.NextCursor
	} else {
		data["nextCursor"] = nil
	}
	httpx.Success(w, http.StatusOK, data)
}

func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	evt, err := h.provider.ParseWebhook(r)
	if err != nil {
		if errors.Is(err, provider.ErrInvalidSignature) {
			httpx.Error(w, r, apperr.New("INVALID_WEBHOOK_SIGNATURE", "Webhook signature verification failed", 401))
			return
		}
		httpx.Error(w, r, apperr.New("INVALID_WEBHOOK_PAYLOAD", "Webhook payload is not valid", 400))
		return
	}
	if err := h.svc.HandleWebhook(r.Context(), evt); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusAccepted, map[string]any{"received": true})
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
