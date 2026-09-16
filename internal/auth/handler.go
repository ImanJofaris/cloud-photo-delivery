package auth

import (
	"net/http"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/audit"
	"github.com/imanjofaris/cloud-photo-delivery/internal/users"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
)

type userDTO struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	BusinessName string `json:"businessName"`
}

type authResponse struct {
	AccessToken  string  `json:"accessToken"`
	RefreshToken string  `json:"refreshToken"`
	ExpiresIn    int     `json:"expiresIn"`
	User         userDTO `json:"user"`
}

func toUserDTO(u *users.User) userDTO {
	return userDTO{ID: u.ID.String(), Email: u.Email, BusinessName: u.BusinessName}
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func decodeJSON(r *http.Request, dst any) error {
	return httpx.DecodeJSON(r, dst, 1<<20)
}

func (h *Handler) toAuthResponse(res *AuthResult) authResponse {
	return authResponse{
		AccessToken:  res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn:    res.ExpiresIn,
		User:         toUserDTO(res.User),
	}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		BusinessName string `json:"businessName"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Signup(r.Context(), in.Email, in.Password, in.BusinessName)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusCreated, h.toAuthResponse(res))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Login(r.Context(), in.Email, in.Password)
	if err != nil {
		audit.Record(r.Context(), "auth.login", "outcome", "failure", "email", in.Email)
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "auth.login",
		"outcome", "success", "user_id", res.User.ID.String(), "email", res.User.Email)
	httpx.Success(w, http.StatusOK, h.toAuthResponse(res))
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	res, err := h.svc.Refresh(r.Context(), in.RefreshToken)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, h.toAuthResponse(res))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refreshToken"`
	}
	// Body is optional; ignore decode errors.
	_ = decodeJSON(r, &in)
	if err := h.svc.Logout(r.Context(), in.RefreshToken); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.RequestPasswordReset(r.Context(), in.Email); err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.Success(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &in); err != nil {
		httpx.Error(w, r, err)
		return
	}
	if err := h.svc.ConfirmPasswordReset(r.Context(), in.Token, in.Password); err != nil {
		httpx.Error(w, r, err)
		return
	}
	audit.Record(r.Context(), "auth.password_reset", "outcome", "success")
	httpx.Success(w, http.StatusOK, map[string]string{"status": "password_reset"})
}
