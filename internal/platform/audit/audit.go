package audit

import (
	"context"
	"log/slog"

	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
)

// ActionKey is the structured field that identifies an audit event.
const ActionKey = "audit_action"

// Record emits one audit event through the request-scoped logger. Handlers
// call it for sensitive actions (login, key rotation, deletes). Callers must
// pass identifiers, never secrets; the logging handler redacts sensitive keys
// as defense in depth.
func Record(ctx context.Context, action string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, ActionKey, action)
	args = append(args, attrs...)
	logging.FromContext(ctx).Info("audit", args...)
}

// RecordWithLogger is used by callers outside an HTTP request (workers) that
// hold a logger directly.
func RecordWithLogger(log *slog.Logger, action string, attrs ...any) {
	args := make([]any, 0, len(attrs)+2)
	args = append(args, ActionKey, action)
	args = append(args, attrs...)
	log.Info("audit", args...)
}
