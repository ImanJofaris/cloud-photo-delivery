package apperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := New("X", "boom", http.StatusBadRequest)
	if got := e.Error(); got != "X: boom" {
		t.Fatalf("unexpected error string: %q", got)
	}
}

func TestError_WithCause(t *testing.T) {
	cause := errors.New("db down")
	e := Internal().WithCause(cause)
	if !errors.Is(e, cause) {
		t.Fatal("expected wrapped cause to be reachable via errors.Is")
	}
}

func TestError_Is(t *testing.T) {
	e := NotFound()
	if !errors.Is(e, ErrNotFound) {
		t.Fatal("expected errors.Is to match ErrNotFound by code")
	}
	if errors.Is(e, ErrConflict) {
		t.Fatal("did not expect match with ErrConflict")
	}
}

func TestCodesAndStatus(t *testing.T) {
	cases := []struct {
		err    *Error
		status int
	}{
		{ErrNotFound, http.StatusNotFound},
		{ErrUnauthorized, http.StatusUnauthorized},
		{ErrForbidden, http.StatusForbidden},
		{ErrConflict, http.StatusConflict},
		{ErrValidation, http.StatusUnprocessableEntity},
		{ErrInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		if tc.err.HTTPStatus != tc.status {
			t.Errorf("%s: got %d want %d", tc.err.Code, tc.err.HTTPStatus, tc.status)
		}
	}
}
