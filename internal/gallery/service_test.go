package gallery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/stretchr/testify/require"
)

func newTestService() (*Service, *fakeRepo, *fakePresigner) {
	svc, repo, presigner, _ := newTestServiceFull()
	return svc, repo, presigner
}

func newTestServiceFull() (*Service, *fakeRepo, *fakePresigner, *fakeBrandingProvider) {
	repo := newFakeRepo()
	presigner := &fakePresigner{}
	branding := &fakeBrandingProvider{}
	urls := photos.NewSignedURLGenerator(presigner, 5*time.Minute)
	tokens := NewUnlockTokens("secret", 30*time.Minute)
	svc := NewService(repo, urls, tokens, func(hash, pw string) bool { return hash == "hash:"+pw }, branding)
	return svc, repo, presigner, branding
}

func codeOf(t *testing.T, err error) string {
	t.Helper()
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	return appErr.Code
}

func TestService_GetEvent_Public(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	ve, requiresUnlock, err := svc.GetEvent(context.Background(), "wedding", "")
	require.NoError(t, err)
	require.False(t, requiresUnlock)
	require.Equal(t, e.ID, ve.Event.ID)
}

func TestService_GetEvent_PrivateHidden(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("secret")
	s.Visibility = events.VisibilityPrivate
	repo.addEvent(e, s)

	_, _, err := svc.GetEvent(context.Background(), "secret", "")
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_GetEvent_UnknownSlug(t *testing.T) {
	svc, _, _ := newTestService()
	_, _, err := svc.GetEvent(context.Background(), "nope", "")
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_GetEvent_PasswordRequiresUnlock(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:open"
	repo.addEvent(e, s)

	_, requiresUnlock, err := svc.GetEvent(context.Background(), "pw", "")
	require.NoError(t, err)
	require.True(t, requiresUnlock)

	token, err := svc.tokens.Issue(e.ID)
	require.NoError(t, err)
	_, requiresUnlock, err = svc.GetEvent(context.Background(), "pw", token)
	require.NoError(t, err)
	require.False(t, requiresUnlock)
}

func TestService_Unlock_WrongPassword(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	_, err := svc.Unlock(context.Background(), "pw", "wrong")
	require.Equal(t, "UNAUTHORIZED", codeOf(t, err))
}

func TestService_Unlock_Success(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	result, err := svc.Unlock(context.Background(), "pw", "right")
	require.NoError(t, err)
	require.NotEmpty(t, result.Token)
	require.Equal(t, 1800, result.ExpiresIn)

	_, err = svc.tokens.Verify(result.Token, e.ID, nil)
	require.NoError(t, err)
}

func TestService_Unlock_NotPasswordProtected(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("open")
	repo.addEvent(e, s)

	_, err := svc.Unlock(context.Background(), "open", "x")
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_Unlock_RevokedAfterPasswordChange(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	result, err := svc.Unlock(context.Background(), "pw", "right")
	require.NoError(t, err)

	// Password changed after the token was issued.
	changed := time.Now().Add(time.Minute)
	s.PasswordChangedAt = &changed

	_, _, err = svc.ListPhotos(context.Background(), "pw", result.Token, "", 0)
	require.Equal(t, "UNAUTHORIZED", codeOf(t, err))
}

func TestService_ListPhotos_Pagination(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		repo.photos[e.ID] = append(repo.photos[e.ID], readyPhoto(e.ID, base.Add(-time.Duration(i)*time.Minute)))
	}
	repo.photos[e.ID] = append(repo.photos[e.ID], &photos.Photo{
		ID: uuid.New(), EventID: e.ID, Status: photos.StatusProcessing, CreatedAt: base,
	})

	_, page, err := svc.ListPhotos(context.Background(), "wedding", "", "", 2)
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	require.NotEmpty(t, page.NextCursor)

	_, page2, err := svc.ListPhotos(context.Background(), "wedding", "", page.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, page2.Items, 2)
	require.NotEmpty(t, page2.NextCursor)

	_, page3, err := svc.ListPhotos(context.Background(), "wedding", "", page2.NextCursor, 2)
	require.NoError(t, err)
	require.Len(t, page3.Items, 1)
	require.Empty(t, page3.NextCursor)

	seen := map[uuid.UUID]bool{}
	for _, p := range append(append(page.Items, page2.Items...), page3.Items...) {
		require.False(t, seen[p.ID], "duplicate photo")
		seen[p.ID] = true
	}
	require.Len(t, seen, 5)
}

func TestService_ListPhotos_InvalidCursor(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	_, _, err := svc.ListPhotos(context.Background(), "wedding", "", "!!!", 0)
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_PhotoURL_SettingsCombinations(t *testing.T) {
	cases := []struct {
		name                  string
		allowDownload         bool
		allowOriginalDownload bool
		variant               string
		wantCode              string
	}{
		{"thumbnail served when downloads off", false, false, "thumbnail", ""},
		{"medium served when downloads off", false, false, "medium", ""},
		{"large served when downloads off", false, false, "large", ""},
		{"original blocked when downloads off", false, true, "original", "DOWNLOAD_DISABLED"},
		{"original blocked when originals off", true, false, "original", "ORIGINAL_DOWNLOAD_DISABLED"},
		{"original served when both on", true, true, "original", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, _ := newTestService()
			e, s := publicEvent("wedding")
			s.AllowDownload = tc.allowDownload
			s.AllowOriginalDownload = tc.allowOriginalDownload
			repo.addEvent(e, s)
			p := readyPhoto(e.ID, time.Now())
			repo.photos[e.ID] = append(repo.photos[e.ID], p)

			res, err := svc.PhotoURL(context.Background(), "wedding", "", p.ID.String(), tc.variant)
			if tc.wantCode == "" {
				require.NoError(t, err)
				require.NotEmpty(t, res.URL)
				return
			}
			require.Equal(t, tc.wantCode, codeOf(t, err))
		})
	}
}

func TestService_PhotoURL_OriginalDisabled(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	s.AllowOriginalDownload = false
	repo.addEvent(e, s)
	p := readyPhoto(e.ID, time.Now())
	repo.photos[e.ID] = append(repo.photos[e.ID], p)

	_, err := svc.PhotoURL(context.Background(), "wedding", "", p.ID.String(), "original")
	require.Equal(t, "ORIGINAL_DOWNLOAD_DISABLED", codeOf(t, err))

	// large still allowed
	res, err := svc.PhotoURL(context.Background(), "wedding", "", p.ID.String(), "large")
	require.NoError(t, err)
	require.Equal(t, *p.OptimizedKey, res.URL[len("https://example.test/"):])
	require.Equal(t, 300, res.ExpiresIn)
}

func TestService_PhotoURL_OriginalAllowedUsesStorageKey(t *testing.T) {
	svc, repo, presigner := newTestService()
	e, s := publicEvent("wedding")
	s.AllowOriginalDownload = true
	repo.addEvent(e, s)
	p := readyPhoto(e.ID, time.Now())
	repo.photos[e.ID] = append(repo.photos[e.ID], p)

	_, err := svc.PhotoURL(context.Background(), "wedding", "", p.ID.String(), "original")
	require.NoError(t, err)
	require.Equal(t, p.StorageKey, presigner.key)
}

func TestService_PhotoURL_InvalidVariant(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	_, err := svc.PhotoURL(context.Background(), "wedding", "", uuid.NewString(), "huge")
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_PhotoURL_NotReady(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	_, err := svc.PhotoURL(context.Background(), "wedding", "", uuid.NewString(), "large")
	require.Equal(t, "PHOTO_NOT_FOUND", codeOf(t, err))
}

func TestService_GetPhoto(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	p := readyPhoto(e.ID, time.Now())
	repo.photos[e.ID] = append(repo.photos[e.ID], p)

	_, got, err := svc.GetPhoto(context.Background(), "wedding", "", p.ID.String())
	require.NoError(t, err)
	require.Equal(t, p.ID, got.ID)
}

func TestService_GetPhoto_InvalidID(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	_, _, err := svc.GetPhoto(context.Background(), "wedding", "", "not-a-uuid")
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_GetPhoto_NotReady(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)

	_, _, err := svc.GetPhoto(context.Background(), "wedding", "", uuid.NewString())
	require.Equal(t, "PHOTO_NOT_FOUND", codeOf(t, err))
}

func TestService_ListPhotos_EmptySlug(t *testing.T) {
	svc, _, _ := newTestService()
	_, _, err := svc.ListPhotos(context.Background(), "  ", "", "", 0)
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_Unlock_EmptyPassword(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	_, err := svc.Unlock(context.Background(), "pw", "")
	require.Equal(t, "VALIDATION_ERROR", codeOf(t, err))
}

func TestService_Unlock_PrivateHidden(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("secret")
	s.Visibility = events.VisibilityPrivate
	repo.addEvent(e, s)

	_, err := svc.Unlock(context.Background(), "secret", "x")
	require.Equal(t, "EVENT_NOT_FOUND", codeOf(t, err))
}

func TestService_VisibleEvent_WrongUnlockToken(t *testing.T) {
	svc, repo, _ := newTestService()
	e, s := publicEvent("pw")
	s.Visibility = events.VisibilityPassword
	s.PasswordHash = "hash:right"
	repo.addEvent(e, s)

	otherToken, err := svc.tokens.Issue(uuid.New())
	require.NoError(t, err)

	_, _, err = svc.ListPhotos(context.Background(), "pw", otherToken, "", 0)
	require.Equal(t, "UNAUTHORIZED", codeOf(t, err))
}

func TestService_VariantsFor(t *testing.T) {
	p := readyPhoto(uuid.New(), time.Now())
	require.Equal(t, []Variant{VariantThumbnail, VariantMedium, VariantLarge}, VariantsFor(p))

	p.ThumbnailKey = nil
	require.Equal(t, []Variant{VariantMedium, VariantLarge}, VariantsFor(p))
}

func TestService_GetEvent_AttachesBranding(t *testing.T) {
	svc, repo, _, branding := newTestServiceFull()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	branding.view = &users.BrandingView{
		BusinessName: "Booth Co",
		PrimaryColor: "#112233",
		LogoURL:      "https://example.test/logo.png",
	}

	ve, _, err := svc.GetEvent(context.Background(), "wedding", "")
	require.NoError(t, err)
	require.NotNil(t, ve.Branding)
	require.Equal(t, "Booth Co", ve.Branding.BusinessName)
	require.Equal(t, e.UserID, branding.userID)
}

func TestService_GetEvent_BrandingErrorIsInternal(t *testing.T) {
	svc, repo, _, branding := newTestServiceFull()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	branding.err = errors.New("db down")

	_, _, err := svc.GetEvent(context.Background(), "wedding", "")
	require.Equal(t, "INTERNAL_ERROR", codeOf(t, err))
}

func TestService_ListPhotos_AttachesBranding(t *testing.T) {
	svc, repo, _, branding := newTestServiceFull()
	e, s := publicEvent("wedding")
	repo.addEvent(e, s)
	repo.photos[e.ID] = append(repo.photos[e.ID], readyPhoto(e.ID, time.Now()))
	branding.view = &users.BrandingView{BusinessName: "Booth Co"}

	ve, _, err := svc.ListPhotos(context.Background(), "wedding", "", "", 0)
	require.NoError(t, err)
	require.NotNil(t, ve.Branding)
	require.Equal(t, "Booth Co", ve.Branding.BusinessName)
}
