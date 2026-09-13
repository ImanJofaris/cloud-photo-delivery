package users

import (
	"context"
	"errors"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const (
	maxBrandingNameLength = 255
	maxContactEmailLength = 320
	maxContactPhoneLength = 40
	maxWebsiteURLLength   = 320

	BrandingAssetLogo         = "logo"
	BrandingAssetProfileImage = "profileImage"
)

type Branding struct {
	UserID          uuid.UUID
	BusinessName    string
	LogoKey         string
	ProfileImageKey string
	PrimaryColor    string
	SecondaryColor  string
	ContactEmail    string
	ContactPhone    string
	WebsiteURL      string
	UpdatedAt       time.Time
}

// BrandingInput is a partial update; nil fields are left unchanged.
type BrandingInput struct {
	BusinessName    *string
	LogoKey         *string
	ProfileImageKey *string
	PrimaryColor    *string
	SecondaryColor  *string
	ContactEmail    *string
	ContactPhone    *string
	WebsiteURL      *string
}

// BrandingView is the API projection: asset keys are replaced by signed URLs.
type BrandingView struct {
	BusinessName    string
	LogoURL         string
	ProfileImageURL string
	PrimaryColor    string
	SecondaryColor  string
	ContactEmail    string
	ContactPhone    string
	WebsiteURL      string
	UpdatedAt       time.Time
}

type AssetUpload struct {
	UploadURL  string
	StorageKey string
	ExpiresAt  time.Time
}

type AssetPresigner interface {
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
}

type BrandingRepository interface {
	GetBranding(ctx context.Context, userID uuid.UUID) (*Branding, error)
	UpsertBranding(ctx context.Context, b Branding) (*Branding, error)
}

type BrandingService struct {
	repo  BrandingRepository
	store AssetPresigner
	ttl   time.Duration
	now   func() time.Time
}

func NewBrandingService(repo BrandingRepository, store AssetPresigner, ttl time.Duration) *BrandingService {
	return &BrandingService{repo: repo, store: store, ttl: ttl, now: time.Now}
}

func (s *BrandingService) SetClock(now func() time.Time) { s.now = now }

func brandingValidation(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}

func (s *BrandingService) Get(ctx context.Context, userID uuid.UUID) (*BrandingView, error) {
	b, err := s.repo.GetBranding(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, apperr.Internal().WithCause(err)
		}
		b = &Branding{UserID: userID}
	}
	return s.signView(ctx, b)
}

func (s *BrandingService) Update(ctx context.Context, userID uuid.UUID, in BrandingInput) (*BrandingView, error) {
	if err := validateBrandingInput(userID, &in); err != nil {
		return nil, err
	}
	current, err := s.repo.GetBranding(ctx, userID)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			return nil, apperr.Internal().WithCause(err)
		}
		current = &Branding{UserID: userID}
	}
	applyBrandingInput(current, &in)
	saved, err := s.repo.UpsertBranding(ctx, *current)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return s.signView(ctx, saved)
}

var brandingAssetExtensions = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

func (s *BrandingService) CreateAssetUpload(ctx context.Context, userID uuid.UUID, kind, contentType string) (*AssetUpload, error) {
	kind = strings.TrimSpace(kind)
	if kind != BrandingAssetLogo && kind != BrandingAssetProfileImage {
		return nil, brandingValidation("kind must be logo or profileImage")
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	ext, ok := brandingAssetExtensions[contentType]
	if !ok {
		return nil, brandingValidation("contentType must be image/png, image/jpeg, or image/webp")
	}
	key := r2.BrandingAssetKey(userID, kind, ext)
	uploadURL, err := s.store.PresignPut(ctx, key, contentType, s.ttl)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	return &AssetUpload{
		UploadURL:  uploadURL,
		StorageKey: key,
		ExpiresAt:  s.now().UTC().Add(s.ttl),
	}, nil
}

func (s *BrandingService) signView(ctx context.Context, b *Branding) (*BrandingView, error) {
	view := &BrandingView{
		BusinessName:   b.BusinessName,
		PrimaryColor:   b.PrimaryColor,
		SecondaryColor: b.SecondaryColor,
		ContactEmail:   b.ContactEmail,
		ContactPhone:   b.ContactPhone,
		WebsiteURL:     b.WebsiteURL,
		UpdatedAt:      b.UpdatedAt,
	}
	if b.LogoKey != "" {
		u, err := s.store.PresignGet(ctx, b.LogoKey, s.ttl)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		view.LogoURL = u
	}
	if b.ProfileImageKey != "" {
		u, err := s.store.PresignGet(ctx, b.ProfileImageKey, s.ttl)
		if err != nil {
			return nil, apperr.Internal().WithCause(err)
		}
		view.ProfileImageURL = u
	}
	return view, nil
}

func validateBrandingInput(userID uuid.UUID, in *BrandingInput) error {
	if in.BusinessName != nil {
		name := strings.TrimSpace(*in.BusinessName)
		if len(name) > maxBrandingNameLength {
			return brandingValidation("Business name must be 255 characters or fewer")
		}
		in.BusinessName = &name
	}
	if in.PrimaryColor != nil {
		color, err := normalizeHexColor(*in.PrimaryColor)
		if err != nil {
			return err
		}
		in.PrimaryColor = &color
	}
	if in.SecondaryColor != nil {
		color, err := normalizeHexColor(*in.SecondaryColor)
		if err != nil {
			return err
		}
		in.SecondaryColor = &color
	}
	if in.ContactEmail != nil {
		email := strings.TrimSpace(*in.ContactEmail)
		if len(email) > maxContactEmailLength {
			return brandingValidation("Contact email must be 320 characters or fewer")
		}
		if email != "" && !validContactEmail(email) {
			return brandingValidation("Contact email is invalid")
		}
		in.ContactEmail = &email
	}
	if in.ContactPhone != nil {
		phone := strings.TrimSpace(*in.ContactPhone)
		if len(phone) > maxContactPhoneLength {
			return brandingValidation("Contact phone must be 40 characters or fewer")
		}
		in.ContactPhone = &phone
	}
	if in.WebsiteURL != nil {
		website := strings.TrimSpace(*in.WebsiteURL)
		if len(website) > maxWebsiteURLLength {
			return brandingValidation("Website URL must be 320 characters or fewer")
		}
		if website != "" && !validWebsiteURL(website) {
			return brandingValidation("Website URL is invalid")
		}
		in.WebsiteURL = &website
	}
	for _, key := range []*string{in.LogoKey, in.ProfileImageKey} {
		if key != nil && *key != "" && !validBrandingKey(userID, *key) {
			return brandingValidation("Invalid branding asset key")
		}
	}
	return nil
}

func applyBrandingInput(b *Branding, in *BrandingInput) {
	if in.BusinessName != nil {
		b.BusinessName = *in.BusinessName
	}
	if in.LogoKey != nil {
		b.LogoKey = *in.LogoKey
	}
	if in.ProfileImageKey != nil {
		b.ProfileImageKey = *in.ProfileImageKey
	}
	if in.PrimaryColor != nil {
		b.PrimaryColor = *in.PrimaryColor
	}
	if in.SecondaryColor != nil {
		b.SecondaryColor = *in.SecondaryColor
	}
	if in.ContactEmail != nil {
		b.ContactEmail = *in.ContactEmail
	}
	if in.ContactPhone != nil {
		b.ContactPhone = *in.ContactPhone
	}
	if in.WebsiteURL != nil {
		b.WebsiteURL = *in.WebsiteURL
	}
}

func normalizeHexColor(raw string) (string, error) {
	color := strings.ToLower(strings.TrimSpace(raw))
	if color == "" {
		return "", nil
	}
	if len(color) != 7 || color[0] != '#' {
		return "", brandingValidation("Brand colors must be hex values like #1a2b3c")
	}
	for i := 1; i < len(color); i++ {
		c := color[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", brandingValidation("Brand colors must be hex values like #1a2b3c")
		}
	}
	return color, nil
}

func validContactEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validWebsiteURL(raw string) bool {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func validBrandingKey(userID uuid.UUID, key string) bool {
	prefix := "tenant/" + userID.String() + "/branding/"
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	rest := strings.TrimPrefix(key, prefix)
	return strings.Contains(rest, "/") && !strings.Contains(rest, "..")
}
