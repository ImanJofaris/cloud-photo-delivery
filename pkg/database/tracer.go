package database

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const maxSlowQuerySQL = 500

type queryTrace struct {
	start time.Time
	sql   string
}

type slowQueryTracer struct {
	threshold time.Duration
	log       *slog.Logger
}

// NewSlowQueryTracer logs queries that run longer than threshold at WARN.
// Arguments are intentionally never logged: they can contain tokens, emails,
// and other sensitive values.
func NewSlowQueryTracer(log *slog.Logger, threshold time.Duration) pgx.QueryTracer {
	return &slowQueryTracer{threshold: threshold, log: log}
}

func (t *slowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTraceKey{}, queryTrace{start: time.Now(), sql: data.SQL})
}

func (t *slowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	trace, ok := ctx.Value(queryTraceKey{}).(queryTrace)
	if !ok {
		return
	}
	d := time.Since(trace.start)
	if d < t.threshold {
		return
	}
	attrs := []any{
		"duration_ms", d.Milliseconds(),
		"sql", truncateSQL(trace.sql),
	}
	if data.Err != nil {
		attrs = append(attrs, "error", data.Err)
	}
	t.log.Warn("slow query", attrs...)
}

func truncateSQL(sql string) string {
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) <= maxSlowQuerySQL {
		return sql
	}
	return sql[:maxSlowQuerySQL] + "..."
}

type queryTraceKey struct{}
