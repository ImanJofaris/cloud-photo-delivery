package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Pool struct {
	*pgxpool.Pool
}

// Options tunes the connection pool. Zero values keep the corresponding pgx
// default; set StatementTimeout to 0 to disable the server-side timeout.
type Options struct {
	MaxConns           int
	MinConns           int
	MaxConnLifetime    time.Duration
	MaxConnIdleTime    time.Duration
	StatementTimeout   time.Duration
	SlowQueryThreshold time.Duration
	Logger             *slog.Logger
}

func DefaultOptions() Options {
	return Options{
		MaxConns:           10,
		MinConns:           1,
		MaxConnLifetime:    time.Hour,
		MaxConnIdleTime:    30 * time.Minute,
		StatementTimeout:   30 * time.Second,
		SlowQueryThreshold: 500 * time.Millisecond,
	}
}

func New(ctx context.Context, databaseURL string) (*Pool, error) {
	return NewWithOptions(ctx, databaseURL, DefaultOptions())
}

func NewWithOptions(ctx context.Context, databaseURL string, opts Options) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	applyPoolOptions(cfg, opts)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Pool{Pool: pool}, nil
}

func applyPoolOptions(cfg *pgxpool.Config, opts Options) {
	if opts.MaxConns > 0 {
		cfg.MaxConns = int32(opts.MaxConns)
	}
	if opts.MinConns >= 0 {
		cfg.MinConns = int32(opts.MinConns)
	}
	if opts.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = opts.MaxConnLifetime
	}
	if opts.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	}
	if opts.StatementTimeout > 0 {
		if cfg.ConnConfig.RuntimeParams == nil {
			cfg.ConnConfig.RuntimeParams = map[string]string{}
		}
		cfg.ConnConfig.RuntimeParams["statement_timeout"] =
			fmt.Sprintf("%d", opts.StatementTimeout.Milliseconds())
	}
	if opts.SlowQueryThreshold > 0 && opts.Logger != nil {
		cfg.ConnConfig.Tracer = NewSlowQueryTracer(opts.Logger, opts.SlowQueryThreshold)
	}
}

func (p *Pool) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}
