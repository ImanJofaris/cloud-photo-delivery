package exports

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("export not found")

type Repository interface {
	// Create inserts a pending export for the event.
	Create(ctx context.Context, eventID uuid.UUID) (*Export, error)
	// GetByID is the worker lookup and is not tenant-scoped.
	GetByID(ctx context.Context, id uuid.UUID) (*Export, error)
	// GetOwned scopes the export to the owner of a non-deleted event.
	GetOwned(ctx context.Context, userID, eventID, exportID uuid.UUID) (*Export, error)
	// GetActiveByEvent returns the newest pending/processing export, if any.
	GetActiveByEvent(ctx context.Context, eventID uuid.UUID) (*Export, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	MarkReady(ctx context.Context, id uuid.UUID, objectKey string, size int64, expiresAt time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, message string) error
	MarkExpired(ctx context.Context, id uuid.UUID) error
	ListExpired(ctx context.Context, now time.Time, limit int) ([]*Export, error)
	ListStaleProcessing(ctx context.Context, before time.Time, limit int) ([]*Export, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const exportColumns = `id, event_id, status, object_key, file_size, error_message,
	expires_at, created_at, updated_at`

func scanExport(row pgx.Row) (*Export, error) {
	var e Export
	err := row.Scan(&e.ID, &e.EventID, &e.Status, &e.ObjectKey, &e.FileSize,
		&e.ErrorMessage, &e.ExpiresAt, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func (r *PostgresRepository) Create(ctx context.Context, eventID uuid.UUID) (*Export, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO exports (id, event_id, status) VALUES ($1, $2, $3)
		 RETURNING `+exportColumns, uuid.New(), eventID, StatusPending)
	return scanExport(row)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Export, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+exportColumns+` FROM exports WHERE id = $1`, id)
	return scanExport(row)
}

func (r *PostgresRepository) GetOwned(ctx context.Context, userID, eventID, exportID uuid.UUID) (*Export, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT x.id, x.event_id, x.status, x.object_key, x.file_size, x.error_message,
			x.expires_at, x.created_at, x.updated_at
		 FROM exports x
		 JOIN events e ON e.id = x.event_id
		 WHERE x.id = $1 AND x.event_id = $2 AND e.user_id = $3 AND e.deleted_at IS NULL`,
		exportID, eventID, userID)
	return scanExport(row)
}

func (r *PostgresRepository) GetActiveByEvent(ctx context.Context, eventID uuid.UUID) (*Export, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+exportColumns+` FROM exports
		 WHERE event_id = $1 AND status IN ($2, $3)
		 ORDER BY created_at DESC, id DESC LIMIT 1`, eventID, StatusPending, StatusProcessing)
	return scanExport(row)
}

func (r *PostgresRepository) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exports SET status = $2, updated_at = NOW()
		 WHERE id = $1 AND status IN ($3, $4, $5)`,
		id, StatusProcessing, StatusPending, StatusProcessing, StatusFailed)
	return err
}

func (r *PostgresRepository) MarkReady(ctx context.Context, id uuid.UUID, objectKey string, size int64, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exports SET status = $2, object_key = $3, file_size = $4,
			expires_at = $5, error_message = NULL, updated_at = NOW()
		 WHERE id = $1`, id, StatusReady, objectKey, size, expiresAt)
	return err
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exports SET status = $2, error_message = $3, updated_at = NOW()
		 WHERE id = $1 AND status IN ($4, $5)`,
		id, StatusFailed, message, StatusPending, StatusProcessing)
	return err
}

func (r *PostgresRepository) MarkExpired(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE exports SET status = $2, updated_at = NOW()
		 WHERE id = $1 AND status = $3`, id, StatusExpired, StatusReady)
	return err
}

func (r *PostgresRepository) ListExpired(ctx context.Context, now time.Time, limit int) ([]*Export, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+exportColumns+` FROM exports
		 WHERE status = $1 AND expires_at IS NOT NULL AND expires_at <= $2
		 ORDER BY expires_at LIMIT $3`, StatusReady, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Export
	for rows.Next() {
		e, err := scanExport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ListStaleProcessing(ctx context.Context, before time.Time, limit int) ([]*Export, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+exportColumns+` FROM exports
		 WHERE status = $1 AND updated_at <= $2
		 ORDER BY updated_at LIMIT $3`, StatusProcessing, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Export
	for rows.Next() {
		e, err := scanExport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
