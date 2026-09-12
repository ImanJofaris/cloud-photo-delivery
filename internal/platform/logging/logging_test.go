package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestNew_Levels(t *testing.T) {
	cases := []struct {
		level    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
	}
	for _, tc := range cases {
		logger := New("dev", tc.level)
		if !logger.Enabled(context.Background(), tc.expected) {
			t.Errorf("level %q: expected enabled at %v", tc.level, tc.expected)
		}
	}
}

func TestFromContext_Default(t *testing.T) {
	if FromContext(context.Background()) == nil {
		t.Fatal("expected non-nil default logger")
	}
}

func TestWith_RoundTrip(t *testing.T) {
	logger := New("dev", "info")
	ctx := With(context.Background(), logger)
	if got := FromContext(ctx); got != logger {
		t.Fatal("expected logger to round-trip through context")
	}
}
