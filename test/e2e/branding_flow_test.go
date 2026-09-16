//go:build integration

package e2e_test

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"testing"

	"github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stretchr/testify/require"
)

func decodeQRPNG(t *testing.T, data []byte) string {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	require.NoError(t, err)
	result, err := gozxingqr.NewQRCodeReader().Decode(bitmap, nil)
	require.NoError(t, err)
	return result.GetText()
}

func TestE2E_BrandingAndQRFlow(t *testing.T) {
	h, pool, _ := setupGalleryAPI(t)

	rec := doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"brand@example.com","password":"password123"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	access, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Branded Event"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code)
	eventID := extractEventID(t, rec)
	slug := eventSlug(t, pool, eventID)
	wantURL := "http://localhost:3000/e/" + slug

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/url", "", access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	var urlEnv struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &urlEnv))
	require.Equal(t, wantURL, urlEnv.Data.URL)

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/qr.png?size=512", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Equal(t, wantURL, decodeQRPNG(t, rec.Body.Bytes()))

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/qr.svg", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "<svg")

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/qr.png", "", "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = doReq(t, h, http.MethodPost, "/api/v1/auth/signup",
		`{"email":"other@example.com","password":"password123"}`, "")
	require.Equal(t, http.StatusCreated, rec.Code)
	otherAccess, _ := parseAuth(t, rec)

	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/qr.png", "", otherAccess)
	require.Equal(t, http.StatusNotFound, rec.Code)
	rec = doReq(t, h, http.MethodGet, "/api/v1/events/"+eventID+"/url", "", otherAccess)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = doReq(t, h, http.MethodPatch, "/api/v1/account/branding",
		`{"businessName":"Iman Booth","primaryColor":"#AABBCC","secondaryColor":"#001122","contactEmail":"hello@example.com","contactPhone":"+60123456789","websiteUrl":"https://booth.example.com"}`,
		access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodGet, "/api/v1/account/branding", "", access)
	require.Equal(t, http.StatusOK, rec.Code)
	var brandEnv struct {
		Data struct {
			BusinessName string  `json:"businessName"`
			PrimaryColor string  `json:"primaryColor"`
			WebsiteURL   string  `json:"websiteUrl"`
			LogoURL      *string `json:"logoUrl"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &brandEnv))
	require.Equal(t, "Iman Booth", brandEnv.Data.BusinessName)
	require.Equal(t, "#aabbcc", brandEnv.Data.PrimaryColor)
	require.Equal(t, "https://booth.example.com", brandEnv.Data.WebsiteURL)
	require.Nil(t, brandEnv.Data.LogoURL)

	rec = doReq(t, h, http.MethodPost, "/api/v1/account/branding/assets",
		`{"kind":"logo","contentType":"image/png"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	var assetEnv struct {
		Data struct {
			StorageKey string `json:"storageKey"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &assetEnv))
	require.NotEmpty(t, assetEnv.Data.StorageKey)

	rec = doReq(t, h, http.MethodPatch, "/api/v1/account/branding",
		`{"logoKey":"`+assetEnv.Data.StorageKey+`"}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodPost, "/api/v1/account/branding/assets",
		`{"kind":"profileImage","contentType":"image/png"}`, access)
	require.Equal(t, http.StatusCreated, rec.Code, "body=%s", rec.Body.String())
	var profileAssetEnv struct {
		Data struct {
			StorageKey string `json:"storageKey"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &profileAssetEnv))
	require.NotEmpty(t, profileAssetEnv.Data.StorageKey)

	rec = doReq(t, h, http.MethodPatch, "/api/v1/account/branding",
		`{"profileImageKey":"`+profileAssetEnv.Data.StorageKey+`"}`, access)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+slug, "", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
	var publicEnv struct {
		Data struct {
			Branding *struct {
				BusinessName    *string `json:"businessName"`
				LogoURL         *string `json:"logoUrl"`
				ProfileImageURL *string `json:"profileImageUrl"`
				PrimaryColor    *string `json:"primaryColor"`
				WebsiteURL      *string `json:"websiteUrl"`
			} `json:"branding"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &publicEnv))
	require.NotNil(t, publicEnv.Data.Branding)
	require.Equal(t, "Iman Booth", *publicEnv.Data.Branding.BusinessName)
	require.Equal(t, "#aabbcc", *publicEnv.Data.Branding.PrimaryColor)
	require.Equal(t, "https://booth.example.com", *publicEnv.Data.Branding.WebsiteURL)
	require.NotEmpty(t, *publicEnv.Data.Branding.LogoURL)
	require.NotEmpty(t, *publicEnv.Data.Branding.ProfileImageURL)

	rec = doReq(t, h, http.MethodPost, "/api/v1/events", `{"name":"Other Event"}`, otherAccess)
	require.Equal(t, http.StatusCreated, rec.Code)
	otherEventID := extractEventID(t, rec)
	otherSlug := eventSlug(t, pool, otherEventID)
	rec = doReq(t, h, http.MethodGet, "/api/v1/public/events/"+otherSlug, "", "")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "Iman Booth")
}
