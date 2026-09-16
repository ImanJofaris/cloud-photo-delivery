package database

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	require.Equal(t, 10, opts.MaxConns)
	require.Equal(t, 1, opts.MinConns)
	require.Equal(t, time.Hour, opts.MaxConnLifetime)
	require.Equal(t, 30*time.Minute, opts.MaxConnIdleTime)
	require.Equal(t, 30*time.Second, opts.StatementTimeout)
	require.Equal(t, 500*time.Millisecond, opts.SlowQueryThreshold)
}

func TestApplyPoolOptions(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://x:y@127.0.0.1:1/none")
	require.NoError(t, err)

	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	applyPoolOptions(cfg, Options{
		MaxConns:           25,
		MinConns:           3,
		MaxConnLifetime:    2 * time.Hour,
		MaxConnIdleTime:    10 * time.Minute,
		StatementTimeout:   1500 * time.Millisecond,
		SlowQueryThreshold: time.Second,
		Logger:             log,
	})

	require.Equal(t, int32(25), cfg.MaxConns)
	require.Equal(t, int32(3), cfg.MinConns)
	require.Equal(t, 2*time.Hour, cfg.MaxConnLifetime)
	require.Equal(t, 10*time.Minute, cfg.MaxConnIdleTime)
	require.Equal(t, "1500", cfg.ConnConfig.RuntimeParams["statement_timeout"])
	require.NotNil(t, cfg.ConnConfig.Tracer)
}

func TestApplyPoolOptions_ZeroKeepsDefaults(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://x:y@127.0.0.1:1/none")
	require.NoError(t, err)
	before := cfg.MaxConns

	applyPoolOptions(cfg, Options{})

	require.Equal(t, before, cfg.MaxConns)
	require.Empty(t, cfg.ConnConfig.RuntimeParams["statement_timeout"])
	require.Nil(t, cfg.ConnConfig.Tracer)
}

func TestApplyPoolOptions_ZeroStatementTimeoutDisables(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://x:y@127.0.0.1:1/none")
	require.NoError(t, err)

	applyPoolOptions(cfg, Options{StatementTimeout: 0})

	require.Empty(t, cfg.ConnConfig.RuntimeParams["statement_timeout"])
}

func TestSlowQueryTracer_LogsSlowQueryWithoutArgs(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	tracer := NewSlowQueryTracer(log, 5*time.Millisecond)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL:  "SELECT * FROM users WHERE token = $1",
		Args: []any{"super-secret-token"},
	})
	time.Sleep(15 * time.Millisecond)
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	out := buf.String()
	require.Contains(t, out, "slow query")
	require.Contains(t, out, "SELECT * FROM users WHERE token = $1")
	require.NotContains(t, out, "super-secret-token")
}

func TestSlowQueryTracer_IgnoresFastQuery(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	tracer := NewSlowQueryTracer(log, time.Hour)

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})

	require.Empty(t, buf.String())
}

func TestSlowQueryTracer_IgnoresMissingStartContext(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	tracer := NewSlowQueryTracer(log, time.Nanosecond)

	tracer.TraceQueryEnd(context.Background(), nil, pgx.TraceQueryEndData{})

	require.Empty(t, buf.String())
}

func TestTruncateSQL(t *testing.T) {
	require.Equal(t, "SELECT 1", truncateSQL("SELECT\n\t1"))

	long := strings.Repeat("x", maxSlowQuerySQL+50)
	got := truncateSQL(long)
	require.Len(t, got, maxSlowQuerySQL+3)
	require.True(t, strings.HasSuffix(got, "..."))
}

func TestNewWithOptions_InvalidDSN(t *testing.T) {
	_, err := NewWithOptions(context.Background(), "not-a-dsn", DefaultOptions())
	require.Error(t, err)
}
