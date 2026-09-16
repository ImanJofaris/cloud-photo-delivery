package logging

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
)

const redacted = "[REDACTED]"

var sensitiveKeyParts = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"authorization",
	"cookie",
	"api_key",
	"apikey",
	"signature",
	"credential",
}

var (
	jwtPattern = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\b`)
	// Signed URL query parameters (S3/R2 presigned URLs).
	signedParamPattern = regexp.MustCompile(`(?i)\b(X-Amz-Signature|X-Amz-Credential|X-Amz-Security-Token|X-Amz-SignedHeaders)=([^&\s"']+)`)
	keyValuePattern    = regexp.MustCompile(`(?i)\b(access_token|refresh_token|password|secret|api_key|apikey|token)=([^&\s"']+)`)
)

// redactHandler scrubs secrets from every record before it reaches the
// underlying handler. It is defense in depth: callers must still never log
// tokens or signed URLs intentionally.
type redactHandler struct {
	next slog.Handler
}

func Redact(next slog.Handler) slog.Handler {
	return redactHandler{next: next}
}

func (h redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h redactHandler) Handle(ctx context.Context, r slog.Record) error {
	clone := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clone.AddAttrs(redactAttr(a))
		return true
	})
	return h.next.Handle(ctx, clone)
}

func (h redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return redactHandler{next: h.next.WithAttrs(out)}
}

func (h redactHandler) WithGroup(name string) slog.Handler {
	return redactHandler{next: h.next.WithGroup(name)}
}

func redactAttr(a slog.Attr) slog.Attr {
	if isSensitiveKey(a.Key) {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		out := make([]slog.Attr, len(group))
		for i, ga := range group {
			out[i] = redactAttr(ga)
		}
		a.Value = slog.GroupValue(out...)
		return a
	}
	if a.Value.Kind() == slog.KindString {
		v := a.Value.String()
		if scrubbed := scrubString(v); scrubbed != v {
			return slog.String(a.Key, scrubbed)
		}
	}
	return a
}

func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, part := range sensitiveKeyParts {
		if strings.Contains(k, part) {
			return true
		}
	}
	return false
}

func scrubString(v string) string {
	if strings.Contains(v, "eyJ") {
		v = jwtPattern.ReplaceAllString(v, redacted)
	}
	v = signedParamPattern.ReplaceAllString(v, "$1="+redacted)
	v = keyValuePattern.ReplaceAllString(v, "$1="+redacted)
	return v
}
