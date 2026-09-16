package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func capture(t *testing.T, fn func(log *slog.Logger)) string {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(Redact(slog.NewJSONHandler(&buf, nil)))
	fn(log)
	return buf.String()
}

func TestRedact_SensitiveKeys(t *testing.T) {
	out := capture(t, func(log *slog.Logger) {
		log.Info("login",
			"password", "hunter2",
			"refresh_token", "opaque-token-value",
			"Authorization", "Bearer abc.def.ghi",
			"api_key", "cpd_live_123",
			"email", "operator@example.com",
		)
	})

	for _, secret := range []string{"hunter2", "opaque-token-value", "Bearer abc.def.ghi", "cpd_live_123"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked into log: %s", secret, out)
		}
	}
	if !strings.Contains(out, "operator@example.com") {
		t.Fatalf("expected non-secret attribute to survive: %s", out)
	}
	if strings.Count(out, redacted) < 4 {
		t.Fatalf("expected four redactions, got: %s", out)
	}
}

func TestRedact_JWTInStringValue(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	out := capture(t, func(log *slog.Logger) {
		log.Info("token issued", "detail", "access="+jwt)
	})

	if strings.Contains(out, jwt) {
		t.Fatalf("JWT leaked into log: %s", out)
	}
	if !strings.Contains(out, redacted) {
		t.Fatalf("expected redaction marker: %s", out)
	}
}

func TestRedact_SignedURLParams(t *testing.T) {
	url := "https://r2.example.com/tenant/1/originals/a.jpg?X-Amz-Signature=deadbeef&X-Amz-Credential=AKIA%2F20260916&X-Amz-SignedHeaders=host"
	out := capture(t, func(log *slog.Logger) {
		log.Info("presigned", "url", url)
	})

	if strings.Contains(out, "deadbeef") || strings.Contains(out, "AKIA") {
		t.Fatalf("signed URL params leaked into log: %s", out)
	}
	if !strings.Contains(out, "X-Amz-Signature="+redacted) {
		t.Fatalf("expected signature redaction: %s", out)
	}
}

func TestRedact_QueryKeyValue(t *testing.T) {
	out := capture(t, func(log *slog.Logger) {
		log.Info("reset link", "link", "https://app.example.com/reset?token=abc123&email=a@b.c")
	})

	if strings.Contains(out, "abc123") {
		t.Fatalf("token query param leaked into log: %s", out)
	}
	if !strings.Contains(out, "a@b.c") {
		t.Fatalf("expected non-secret query value to survive: %s", out)
	}
}

func TestRedact_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(Redact(slog.NewJSONHandler(&buf, nil)))
	log := base.With("token", "should-not-appear")

	log.Info("hello")

	if strings.Contains(buf.String(), "should-not-appear") {
		t.Fatalf("WithAttrs secret leaked: %s", buf.String())
	}
}

func TestRedact_Groups(t *testing.T) {
	out := capture(t, func(log *slog.Logger) {
		log.Info("ctx", slog.Group("auth", slog.String("password", "nope"), slog.String("user", "alice")))
	})

	if strings.Contains(out, "nope") {
		t.Fatalf("group secret leaked: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Fatalf("expected group field to survive: %s", out)
	}
}

func TestRedact_EnabledDelegates(t *testing.T) {
	handler := Redact(slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if handler.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("expected Enabled to delegate to the wrapped handler")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("expected error level to be enabled")
	}
}
