//go:build integration

package database_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestBackupRestoreRoundTrip proves the pg_dump/pg_restore procedure used by
// scripts/backup.sh and scripts/restore.sh: seed a database, dump it, destroy
// the table, restore, and assert the rows are back.
func TestBackupRestoreRoundTrip(t *testing.T) {
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

	// Exec scripts run inside the container, so they must use the container's
	// own address, not the host-mapped port from ConnectionString.
	const internalDSN = "postgres://cpd:cpd@127.0.0.1:5432/cpd?sslmode=disable"

	exec := func(script string) string {
		t.Helper()
		code, reader, err := pg.Exec(ctx, []string{"sh", "-c", "export PGURL='" + internalDSN + "'; " + script})
		require.NoError(t, err)
		out, _ := io.ReadAll(reader)
		require.Zero(t, code, "script failed: %s\n%s", script, string(out))
		return string(out)
	}

	exec(`psql "$PGURL" -v ON_ERROR_STOP=1 -c "CREATE TABLE sample (id SERIAL PRIMARY KEY, name TEXT NOT NULL)"`)
	exec(`psql "$PGURL" -v ON_ERROR_STOP=1 -c "INSERT INTO sample (name) VALUES ('alpha'), ('beta')"`)

	exec(`pg_dump --format=custom --no-owner --no-acl --file=/tmp/cpd.dump "$PGURL"`)
	exec(`pg_restore --list /tmp/cpd.dump >/dev/null`)

	exec(`psql "$PGURL" -v ON_ERROR_STOP=1 -c "DROP TABLE sample"`)

	exec(`pg_restore --clean --if-exists --no-owner --no-acl --dbname "$PGURL" /tmp/cpd.dump`)

	out := exec(`psql "$PGURL" -tA -c "SELECT string_agg(name, ',' ORDER BY id) FROM sample"`)
	// Exec output is multiplexed with an 8-byte stdcopy header, so match the
	// payload rather than comparing the raw stream.
	require.Contains(t, out, "alpha,beta")
}
