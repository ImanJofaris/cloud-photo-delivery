package audit

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
)

func TestRecord_EmitsStructuredEvent(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(logging.Redact(slog.NewJSONHandler(&buf, nil)))
	ctx := logging.With(context.Background(), log)

	Record(ctx, "device.rotate", "actor_id", "user-1", "device_id", "dev-1")

	out := buf.String()
	if !strings.Contains(out, `"audit_action":"device.rotate"`) {
		t.Fatalf("missing action: %s", out)
	}
	if !strings.Contains(out, `"actor_id":"user-1"`) || !strings.Contains(out, `"device_id":"dev-1"`) {
		t.Fatalf("missing attributes: %s", out)
	}
}

func TestRecord_RedactsSensitiveAttributes(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(logging.Redact(slog.NewJSONHandler(&buf, nil)))
	ctx := logging.With(context.Background(), log)

	Record(ctx, "auth.login", "email", "a@b.c", "password", "hunter2", "access_token", "jwt-value")

	out := buf.String()
	if strings.Contains(out, "hunter2") || strings.Contains(out, "jwt-value") {
		t.Fatalf("audit leaked a secret: %s", out)
	}
	if !strings.Contains(out, "a@b.c") {
		t.Fatalf("expected non-secret attribute: %s", out)
	}
}

func TestRecordWithLogger(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(logging.Redact(slog.NewJSONHandler(&buf, nil)))

	RecordWithLogger(log, "event.purge", "event_id", "evt-1")

	if !strings.Contains(buf.String(), `"audit_action":"event.purge"`) {
		t.Fatalf("missing action: %s", buf.String())
	}
}
