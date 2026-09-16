package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	Data  any        `json:"data"`
	Error *ErrorBody `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func Success(w http.ResponseWriter, status int, data any) {
	WriteJSON(w, status, Envelope{Data: data, Error: nil})
}

func Fail(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, Envelope{Data: nil, Error: &ErrorBody{Code: code, Message: message}})
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		Fail(w, appErr.HTTPStatus, appErr.Code, appErr.Message)
		return
	}

	if errors.Is(err, context.DeadlineExceeded) {
		Fail(w, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "Request timed out")
		return
	}

	log := Logger(r)
	if log == nil {
		log = slog.Default()
	}
	log.Error("unhandled error", "error", err, "path", r.URL.Path)
	Fail(w, http.StatusInternalServerError, apperr.ErrInternal.Code, apperr.ErrInternal.Message)
}
