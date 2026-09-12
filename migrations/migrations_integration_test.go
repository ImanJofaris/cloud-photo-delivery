//go:build integration

package migrations_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestMigrations_UpSQLApplies(t *testing.T) {
	ctx := context.Background()

	pg, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("cpd"),
		postgres.WithUsername("cpd"),
		postgres.WithPassword("cpd"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pg.Terminate(ctx) })

	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	upSQL, err := os.ReadFile("0001_init.sql")
	require.NoError(t, err)

	// Extract the Up section between "-- +goose Up" and "-- +goose Down".
	up := extractSection(string(upSQL), "-- +goose Up", "-- +goose Down")
	require.NotEmpty(t, up)

	_, err = pool.Exec(ctx, up)
	require.NoError(t, err)

	var exists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'schema_migrations_baseline')",
	).Scan(&exists)
	require.NoError(t, err)
	require.True(t, exists)
}

func extractSection(s, start, end string) string {
	i := indexOf(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := indexOf(s[i:], end)
	if j < 0 {
		return s[i:]
	}
	return s[i : i+j]
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
