package gallery

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

const (
	cachePublic  = "public, max-age=60"
	cachePrivate = "private, no-store"
)

type publicEventDTO struct {
	Name                  string             `json:"name"`
	Date                  *string            `json:"date"`
	Location              string             `json:"location"`
	Description           string             `json:"description"`
	Visibility            string             `json:"visibility"`
	AllowDownload         bool               `json:"allowDownload"`
	AllowOriginalDownload bool               `json:"allowOriginalDownload"`
	PhotoCount            int64              `json:"photoCount"`
	CoverPhotoID          *string            `json:"coverPhotoId"`
	RequiresUnlock        bool               `json:"requiresUnlock"`
	Branding              *publicBrandingDTO `json:"branding"`
}

type publicBrandingDTO struct {
	BusinessName   *string `json:"businessName"`
	LogoURL        *string `json:"logoUrl"`
	PrimaryColor   *string `json:"primaryColor"`
	SecondaryColor *string `json:"secondaryColor"`
	ContactEmail   *string `json:"contactEmail"`
	ContactPhone   *string `json:"contactPhone"`
	WebsiteURL     *string `json:"websiteUrl"`
}

type publicPhotoDTO struct {
	ID       string   `json:"id"`
	Width    *int     `json:"width"`
	Height   *int     `json:"height"`
	Variants []string `json:"variants"`
}

type unlockRequest struct {
	Password string `json:"password"`
}

func toPublicEventDTO(e *events.Event, s *events.Settings, requiresUnlock bool, branding *users.BrandingView) publicEventDTO {
	dto := publicEventDTO{
		Name:                  e.Name,
		Location:              e.Location,
		Description:           e.Description,
		Visibility:            string(s.Visibility),
		AllowDownload:         s.AllowDownload,
		AllowOriginalDownload: s.AllowOriginalDownload,
		PhotoCount:            e.PhotoCount,
		RequiresUnlock:        requiresUnlock,
		Branding:              toPublicBrandingDTO(branding),
	}
	if e.EventDate != nil {
		d := e.EventDate.Format("2006-01-02")
		dto.Date = &d
	}
	if e.CoverPhotoID != nil {
		c := e.CoverPhotoID.String()
		dto.CoverPhotoID = &c
	}
	return dto
}

func toPublicBrandingDTO(b *users.BrandingView) *publicBrandingDTO {
	if b == nil {
		return nil
	}
	if b.BusinessName == "" && b.LogoURL == "" && b.PrimaryColor == "" && b.SecondaryColor == "" &&
		b.ContactEmail == "" && b.ContactPhone == "" && b.WebsiteURL == "" {
		return nil
	}
	dto := &publicBrandingDTO{
		BusinessName:   optionalString(b.BusinessName),
		LogoURL:        optionalString(b.LogoURL),
		PrimaryColor:   optionalString(b.PrimaryColor),
		SecondaryColor: optionalString(b.SecondaryColor),
		ContactEmail:   optionalString(b.ContactEmail),
		ContactPhone:   optionalString(b.ContactPhone),
		WebsiteURL:     optionalString(b.WebsiteURL),
	}
	return dto
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toPublicPhotoDTO(p *photos.Photo) publicPhotoDTO {
	variants := VariantsFor(p)
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		out = append(out, string(v))
	}
	return publicPhotoDTO{
		ID:       p.ID.String(),
		Width:    p.Width,
		Height:   p.Height,
		Variants: out,
	}
}

func cacheHeader(w http.ResponseWriter, s *events.Settings) {
	if s.Visibility == events.VisibilityPassword {
		w.Header().Set("Cache-Control", cachePrivate)
		return
	}
	w.Header().Set("Cache-Control", cachePublic)
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func slugParam(r *http.Request) string { return chi.URLParam(r, "slug") }

func unlockHeader(r *http.Request) string { return r.Header.Get("X-Gallery-Unlock") }

func decodeBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return validationError("Request body is not valid JSON")
	}
	return nil
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

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	ve, requiresUnlock, err := h.svc.GetEvent(r.Context(), slugParam(r), unlockHeader(r))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	cacheHeader(w, ve.Settings)
	httpx.Success(w, http.StatusOK, toPublicEventDTO(ve.Event, ve.Settings, requiresUnlock, ve.Branding))
}

func (h *Handler) Unlock(w http.ResponseWriter, r *http.Request) {
	var req unlockRequest
	if err := decodeBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	result, err := h.svc.Unlock(r.Context(), slugParam(r), req.Password)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", cachePrivate)
	httpx.Success(w, http.StatusOK, map[string]any{
		"token":     result.Token,
		"expiresIn": result.ExpiresIn,
	})
}

func (h *Handler) ListPhotos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	ve, page, err := h.svc.ListPhotos(r.Context(), slugParam(r), unlockHeader(r), q.Get("cursor"), parseLimit(q.Get("limit")))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	items := make([]publicPhotoDTO, 0, len(page.Items))
	for _, p := range page.Items {
		items = append(items, toPublicPhotoDTO(p))
	}
	var next any
	if page.NextCursor != "" {
		next = page.NextCursor
	}
	cacheHeader(w, ve.Settings)
	httpx.Success(w, http.StatusOK, map[string]any{
		"event": toPublicEventDTO(ve.Event, ve.Settings, false, ve.Branding),
		"photos": map[string]any{
			"items":      items,
			"nextCursor": next,
		},
	})
}

func (h *Handler) GetPhoto(w http.ResponseWriter, r *http.Request) {
	ve, photo, err := h.svc.GetPhoto(r.Context(), slugParam(r), unlockHeader(r), chi.URLParam(r, "photoID"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	cacheHeader(w, ve.Settings)
	httpx.Success(w, http.StatusOK, toPublicPhotoDTO(photo))
}

func (h *Handler) PhotoURL(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.PhotoURL(r.Context(), slugParam(r), unlockHeader(r), chi.URLParam(r, "photoID"), r.URL.Query().Get("variant"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", cachePrivate)
	httpx.Success(w, http.StatusOK, map[string]any{
		"url":       result.URL,
		"expiresIn": result.ExpiresIn,
	})
}
