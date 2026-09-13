package users

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/stretchr/testify/require"
)

type fakeBrandingRepo struct {
	branding *Branding
	getErr   error
	upserts  int
}

func (f *fakeBrandingRepo) GetBranding(_ context.Context, userID uuid.UUID) (*Branding, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.branding == nil {
		return nil, ErrNotFound
	}
	cp := *f.branding
	return &cp, nil
}

func (f *fakeBrandingRepo) UpsertBranding(_ context.Context, b Branding) (*Branding, error) {
	f.upserts++
	f.branding = &b
	return &b, nil
}

type fakeAssetStore struct {
	getErr          error
	putErr          error
	lastKey         string
	lastContentType string
}

func (f *fakeAssetStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.lastKey = key
	if f.getErr != nil {
		return "", f.getErr
	}
	return "https://assets.test/" + key, nil
}

func (f *fakeAssetStore) PresignPut(_ context.Context, key, contentType string, _ time.Duration) (string, error) {
	f.lastKey = key
	f.lastContentType = contentType
	if f.putErr != nil {
		return "", f.putErr
	}
	return "https://assets.test/put/" + key, nil
}

func brandingCodeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func strPtr(s string) *string { return &s }

func newBrandingService(repo *fakeBrandingRepo, store *fakeAssetStore) *BrandingService {
	svc := NewBrandingService(repo, store, 5*time.Minute)
	svc.SetClock(func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) })
	return svc
}

func TestBrandingService_Get_DefaultsWhenMissing(t *testing.T) {
	svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{})
	view, err := svc.Get(context.Background(), uuid.New())
	require.NoError(t, err)
	require.Empty(t, view.BusinessName)
	require.Empty(t, view.LogoURL)
	require.Empty(t, view.ProfileImageURL)
}

func TestBrandingService_Get_SignsAssetURLs(t *testing.T) {
	userID := uuid.New()
	repo := &fakeBrandingRepo{branding: &Branding{
		UserID:          userID,
		BusinessName:    "Booth Co",
		LogoKey:         "tenant/" + userID.String() + "/branding/logo/a.png",
		ProfileImageKey: "tenant/" + userID.String() + "/branding/profileImage/b.jpg",
		PrimaryColor:    "#112233",
	}}
	svc := newBrandingService(repo, &fakeAssetStore{})

	view, err := svc.Get(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, "https://assets.test/tenant/"+userID.String()+"/branding/profileImage/b.jpg", view.ProfileImageURL)
	require.Equal(t, "https://assets.test/tenant/"+userID.String()+"/branding/logo/a.png", view.LogoURL)
}

func TestBrandingService_Get_SignErrorIsInternal(t *testing.T) {
	repo := &fakeBrandingRepo{branding: &Branding{LogoKey: "tenant/x/branding/logo/a.png"}}
	svc := newBrandingService(repo, &fakeAssetStore{getErr: errors.New("store down")})

	_, err := svc.Get(context.Background(), uuid.New())
	require.Equal(t, "INTERNAL_ERROR", brandingCodeOf(t, err))
}

func TestBrandingService_Update_PartialMerge(t *testing.T) {
	userID := uuid.New()
	name := "Old Booth"
	color := "#112233"
	phone := "+60123456789"
	repo := &fakeBrandingRepo{branding: &Branding{
		UserID:       userID,
		BusinessName: name,
		PrimaryColor: color,
		ContactPhone: phone,
	}}
	svc := newBrandingService(repo, &fakeAssetStore{})

	newName := "New Booth"
	emptyPhone := ""
	view, err := svc.Update(context.Background(), userID, BrandingInput{
		BusinessName: &newName,
		ContactPhone: &emptyPhone,
	})
	require.NoError(t, err)
	require.Equal(t, "New Booth", view.BusinessName)
	require.Equal(t, "#112233", view.PrimaryColor)
	require.Empty(t, view.ContactPhone)
	require.Equal(t, "New Booth", repo.branding.BusinessName)
	require.Equal(t, "#112233", repo.branding.PrimaryColor)
}

func TestBrandingService_Update_InsertsWhenMissing(t *testing.T) {
	userID := uuid.New()
	repo := &fakeBrandingRepo{}
	svc := newBrandingService(repo, &fakeAssetStore{})

	name := "Booth Co"
	view, err := svc.Update(context.Background(), userID, BrandingInput{BusinessName: &name})
	require.NoError(t, err)
	require.Equal(t, "Booth Co", view.BusinessName)
	require.Equal(t, 1, repo.upserts)
}

func TestBrandingService_Update_NormalizesColor(t *testing.T) {
	userID := uuid.New()
	repo := &fakeBrandingRepo{}
	svc := newBrandingService(repo, &fakeAssetStore{})

	color := "  #AABBCC "
	view, err := svc.Update(context.Background(), userID, BrandingInput{PrimaryColor: &color})
	require.NoError(t, err)
	require.Equal(t, "#aabbcc", view.PrimaryColor)
}

func TestBrandingService_Update_Validation(t *testing.T) {
	userID := uuid.New()
	validKey := "tenant/" + userID.String() + "/branding/logo/one.png"
	cases := map[string]BrandingInput{
		"name too long":    {BusinessName: strPtr(strings.Repeat("a", 256))},
		"color missing #":  {PrimaryColor: strPtr("aabbcc")},
		"color short":      {PrimaryColor: strPtr("#abc")},
		"color non-hex":    {PrimaryColor: strPtr("#gggggg")},
		"secondary bad":    {SecondaryColor: strPtr("#12345z")},
		"email invalid":    {ContactEmail: strPtr("not-an-email")},
		"email too long":   {ContactEmail: strPtr(strings.Repeat("a", 310) + "@example.com")},
		"phone too long":   {ContactPhone: strPtr(strings.Repeat("9", 41))},
		"website no host":  {WebsiteURL: strPtr("https://")},
		"website no https": {WebsiteURL: strPtr("ftp://example.com")},
		"foreign logo key": {LogoKey: strPtr("tenant/other/branding/logo/x.png")},
		"foreign path":     {ProfileImageKey: strPtr("tenant/" + userID.String() + "/photos/x.png")},
		"traversal":        {LogoKey: strPtr("tenant/" + userID.String() + "/branding/../evil.png")},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{})
			_, err := svc.Update(context.Background(), userID, in)
			require.Equal(t, "VALIDATION_ERROR", brandingCodeOf(t, err))
		})
	}

	svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{})
	valid := BrandingInput{
		PrimaryColor:    strPtr("#AABBCC"),
		SecondaryColor:  strPtr("#001122"),
		ContactEmail:    strPtr("hello@example.com"),
		ContactPhone:    strPtr("+60123456789"),
		WebsiteURL:      strPtr("https://booth.example.com"),
		LogoKey:         strPtr(validKey),
		ProfileImageKey: strPtr("tenant/" + userID.String() + "/branding/profileImage/p.png"),
	}
	_, err := svc.Update(context.Background(), userID, valid)
	require.NoError(t, err)
}

func TestBrandingService_Update_RepoErrorIsInternal(t *testing.T) {
	repo := &fakeBrandingRepo{getErr: errors.New("db down")}
	svc := newBrandingService(repo, &fakeAssetStore{})
	_, err := svc.Update(context.Background(), uuid.New(), BrandingInput{})
	require.Equal(t, "INTERNAL_ERROR", brandingCodeOf(t, err))
}

func TestBrandingService_CreateAssetUpload(t *testing.T) {
	userID := uuid.New()
	store := &fakeAssetStore{}
	svc := newBrandingService(&fakeBrandingRepo{}, store)

	upload, err := svc.CreateAssetUpload(context.Background(), userID, BrandingAssetLogo, "image/png")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(upload.StorageKey, "tenant/"+userID.String()+"/branding/logo/"), upload.StorageKey)
	require.True(t, strings.HasSuffix(upload.StorageKey, ".png"), upload.StorageKey)
	require.Equal(t, "image/png", store.lastContentType)
	require.Contains(t, upload.UploadURL, upload.StorageKey)
	require.Equal(t, time.Date(2026, 1, 2, 3, 9, 5, 0, time.UTC), upload.ExpiresAt)

	profile, err := svc.CreateAssetUpload(context.Background(), userID, BrandingAssetProfileImage, "image/jpeg")
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(profile.StorageKey, ".jpg"), profile.StorageKey)
}

func TestBrandingService_CreateAssetUpload_Validation(t *testing.T) {
	svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{})

	_, err := svc.CreateAssetUpload(context.Background(), uuid.New(), "avatar", "image/png")
	require.Equal(t, "VALIDATION_ERROR", brandingCodeOf(t, err))

	_, err = svc.CreateAssetUpload(context.Background(), uuid.New(), BrandingAssetLogo, "image/gif")
	require.Equal(t, "VALIDATION_ERROR", brandingCodeOf(t, err))
}

func TestBrandingService_CreateAssetUpload_PresignErrorIsInternal(t *testing.T) {
	svc := newBrandingService(&fakeBrandingRepo{}, &fakeAssetStore{putErr: errors.New("store down")})
	_, err := svc.CreateAssetUpload(context.Background(), uuid.New(), BrandingAssetLogo, "image/webp")
	require.Equal(t, "INTERNAL_ERROR", brandingCodeOf(t, err))
}

func TestValidWebsiteURL(t *testing.T) {
	require.True(t, validWebsiteURL("https://example.com/path?q=1"))
	require.True(t, validWebsiteURL("http://example.com"))
	require.False(t, validWebsiteURL("example.com"))
	require.False(t, validWebsiteURL("javascript:alert(1)"))
	require.False(t, validWebsiteURL("https://"))
}

func TestBrandingR2KeyHelpers(t *testing.T) {
	userID := uuid.New()
	key := "tenant/" + userID.String() + "/branding/logo/" + uuid.NewString() + ".png"
	require.True(t, validBrandingKey(userID, key))
	require.False(t, validBrandingKey(uuid.New(), key))

	u, err := url.Parse("https://example.com")
	require.NoError(t, err)
	require.Equal(t, "https", u.Scheme)
}
