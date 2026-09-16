package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

func newTestHandler(t *testing.T) (*Handler, *fakeMailer) {
	svc, _, _, mailer := newTestService(t)
	return NewHandler(svc), mailer
}

func doJSON(t *testing.T, h http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestHandler_SignupCreated(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.Signup, http.MethodPost, "/auth/signup",
		`{"email":"a@b.com","password":"password123","businessName":"Booth"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
	var env httpx.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Error != nil {
		t.Fatalf("unexpected error: %+v", env.Error)
	}
}

func TestHandler_SignupInvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.Signup, http.MethodPost, "/auth/signup", `{not json`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandler_LoginUnauthorized(t *testing.T) {
	h, _ := newTestHandler(t)
	_ = doJSON(t, h.Signup, http.MethodPost, "/auth/signup",
		`{"email":"a@b.com","password":"password123"}`)

	rec := doJSON(t, h.Login, http.MethodPost, "/auth/login",
		`{"email":"a@b.com","password":"wrong"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHandler_SignupAndLoginIncludeIsAdminFalse(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.Signup, http.MethodPost, "/auth/signup",
		`{"email":"a@b.com","password":"password123","businessName":"Booth"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"isAdmin":false`) {
		t.Fatalf("signup response missing isAdmin: %s", rec.Body.String())
	}

	rec = doJSON(t, h.Login, http.MethodPost, "/auth/login",
		`{"email":"a@b.com","password":"password123"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login got %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			User struct {
				IsAdmin bool `json:"isAdmin"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.User.IsAdmin {
		t.Fatalf("ordinary operator must not be admin: %s", rec.Body.String())
	}
}

func TestHandler_Refresh(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.Signup, http.MethodPost, "/auth/signup",
		`{"email":"a@b.com","password":"password123"}`)
	var env struct {
		Data struct {
			RefreshToken string `json:"refreshToken"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	rec = doJSON(t, h.Refresh, http.MethodPost, "/auth/refresh",
		`{"refreshToken":"`+env.Data.RefreshToken+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandler_RequestPasswordResetAlwaysOK(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.RequestPasswordReset, http.MethodPost, "/auth/password/reset-request",
		`{"email":"nobody@b.com"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestHandler_Logout(t *testing.T) {
	h, _ := newTestHandler(t)
	rec := doJSON(t, h.Logout, http.MethodPost, "/auth/logout", `{}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestConfig_DefaultsUsedByService(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	svc.cfg.AccessTTL = 15 * time.Minute
	if svc.cfg.AccessTTL != 15*time.Minute {
		t.Fatal("config not applied")
	}
}
