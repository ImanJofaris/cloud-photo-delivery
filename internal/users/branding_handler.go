package users

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type brandingDTO struct {
	BusinessName    *string `json:"businessName"`
	LogoURL         *string `json:"logoUrl"`
	ProfileImageURL *string `json:"profileImageUrl"`
	PrimaryColor    *string `json:"primaryColor"`
	SecondaryColor  *string `json:"secondaryColor"`
	ContactEmail    *string `json:"contactEmail"`
	ContactPhone    *string `json:"contactPhone"`
	WebsiteURL      *string `json:"websiteUrl"`
	UpdatedAt       *string `json:"updatedAt"`
}

type brandingAssetDTO struct {
	UploadURL  string `json:"uploadUrl"`
	StorageKey string `json:"storageKey"`
	ExpiresAt  string `json:"expiresAt"`
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toBrandingDTO(v *BrandingView) brandingDTO {
	dto := brandingDTO{
		BusinessName:    optionalString(v.BusinessName),
		LogoURL:         optionalString(v.LogoURL),
		ProfileImageURL: optionalString(v.ProfileImageURL),
		PrimaryColor:    optionalString(v.PrimaryColor),
		SecondaryColor:  optionalString(v.SecondaryColor),
		ContactEmail:    optionalString(v.ContactEmail),
		ContactPhone:    optionalString(v.ContactPhone),
		WebsiteURL:      optionalString(v.WebsiteURL),
	}
	if !v.UpdatedAt.IsZero() {
		s := v.UpdatedAt.UTC().Format(time.RFC3339)
		dto.UpdatedAt = &s
	}
	return dto
}

type BrandingHandler struct {
	svc   *BrandingService
	getID func(*http.Request) (string, bool)
}

func NewBrandingHandler(svc *BrandingService, resolveUserID func(*http.Request) (string, bool)) *BrandingHandler {
	return &BrandingHandler{svc: svc, getID: resolveUserID}
}

func (h *BrandingHandler) userID(r *http.Request) (uuid.UUID, error) {
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

func decodeBrandingBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return brandingValidation("Request body is not valid JSON")
	}
	return nil
}

func (h *BrandingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBrandingDTO(view))
}

type brandingUpdateRequest struct {
	BusinessName    *string `json:"businessName"`
	LogoKey         *string `json:"logoKey"`
	ProfileImageKey *string `json:"profileImageKey"`
	PrimaryColor    *string `json:"primaryColor"`
	SecondaryColor  *string `json:"secondaryColor"`
	ContactEmail    *string `json:"contactEmail"`
	ContactPhone    *string `json:"contactPhone"`
	WebsiteURL      *string `json:"websiteUrl"`
}

func (h *BrandingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req brandingUpdateRequest
	if err := decodeBrandingBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	view, err := h.svc.Update(r.Context(), id, BrandingInput{
		BusinessName:    req.BusinessName,
		LogoKey:         req.LogoKey,
		ProfileImageKey: req.ProfileImageKey,
		PrimaryColor:    req.PrimaryColor,
		SecondaryColor:  req.SecondaryColor,
		ContactEmail:    req.ContactEmail,
		ContactPhone:    req.ContactPhone,
		WebsiteURL:      req.WebsiteURL,
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, toBrandingDTO(view))
}

type brandingAssetRequest struct {
	Kind        string `json:"kind"`
	ContentType string `json:"contentType"`
}

func (h *BrandingHandler) CreateAssetUpload(w http.ResponseWriter, r *http.Request) {
	id, err := h.userID(r)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	var req brandingAssetRequest
	if err := decodeBrandingBody(r, &req); err != nil {
		httpx.Error(w, r, err)
		return
	}
	upload, err := h.svc.CreateAssetUpload(r.Context(), id, req.Kind, req.ContentType)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusCreated, brandingAssetDTO{
		UploadURL:  upload.UploadURL,
		StorageKey: upload.StorageKey,
		ExpiresAt:  upload.ExpiresAt.Format(time.RFC3339),
	})
}
