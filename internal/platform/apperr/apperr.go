package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
	cause      error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.cause }

func (e *Error) WithCause(err error) *Error {
	c := *e
	c.cause = err
	return &c
}

func New(code, message string, status int) *Error {
	return &Error{Code: code, Message: message, HTTPStatus: status}
}

func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

var (
	ErrNotFound     = New("NOT_FOUND", "Resource not found", http.StatusNotFound)
	ErrUnauthorized = New("UNAUTHORIZED", "Authentication required", http.StatusUnauthorized)
	ErrForbidden    = New("FORBIDDEN", "You do not have access to this resource", http.StatusForbidden)
	ErrConflict     = New("CONFLICT", "Resource conflict", http.StatusConflict)
	ErrValidation   = New("VALIDATION_ERROR", "The request is invalid", http.StatusUnprocessableEntity)
	ErrInternal     = New("INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError)
)

func NotFound() *Error     { return ErrNotFound }
func Unauthorized() *Error { return ErrUnauthorized }
func Forbidden() *Error    { return ErrForbidden }
func Conflict() *Error     { return ErrConflict }
func Validation() *Error   { return ErrValidation }
func Internal() *Error     { return ErrInternal }
