package devices

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("device not found")

const deviceColumns = `id, user_id, name, key_prefix, key_hash,
	assigned_event_id, revoked_at, last_used_at, created_at`

type Repository interface {
	Create(ctx context.Context, d *Device) (*Device, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*Device, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*Device, error)
	GetByPrefix(ctx context.Context, prefix string) (*Device, error)
	Rename(ctx context.Context, userID, id uuid.UUID, name string) (*Device, error)
	Rotate(ctx context.Context, userID, id uuid.UUID, prefix, hash string) (*Device, error)
	Revoke(ctx context.Context, userID, id uuid.UUID, at time.Time) error
	TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error
	EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Create(ctx context.Context, d *Device) (*Device, error) {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO devices (id, user_id, name, key_prefix, key_hash, assigned_event_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		d.ID, d.UserID, d.Name, d.KeyPrefix, d.KeyHash, d.AssignedEventID, d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return d, nil
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*Device, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+deviceColumns+` FROM devices WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*Device{}
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

func (r *PostgresRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*Device, error) {
	return scanDevice(r.pool.QueryRow(ctx,
		`SELECT `+deviceColumns+` FROM devices WHERE id = $1 AND user_id = $2`, id, userID))
}

func (r *PostgresRepository) GetByPrefix(ctx context.Context, prefix string) (*Device, error) {
	return scanDevice(r.pool.QueryRow(ctx,
		`SELECT `+deviceColumns+` FROM devices WHERE key_prefix = $1`, prefix))
}

func (r *PostgresRepository) Rename(ctx context.Context, userID, id uuid.UUID, name string) (*Device, error) {
	return scanDevice(r.pool.QueryRow(ctx,
		`UPDATE devices SET name = $3 WHERE id = $1 AND user_id = $2 RETURNING `+deviceColumns,
		id, userID, name))
}

func (r *PostgresRepository) Rotate(ctx context.Context, userID, id uuid.UUID, prefix, hash string) (*Device, error) {
	return scanDevice(r.pool.QueryRow(ctx,
		`UPDATE devices SET key_prefix = $3, key_hash = $4 WHERE id = $1 AND user_id = $2 RETURNING `+deviceColumns,
		id, userID, prefix, hash))
}

func (r *PostgresRepository) Revoke(ctx context.Context, userID, id uuid.UUID, at time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE devices SET revoked_at = $3 WHERE id = $1 AND user_id = $2`, id, userID, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) TouchLastUsed(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.pool.Exec(ctx, `UPDATE devices SET last_used_at = $2 WHERE id = $1`, id, at)
	return err
}

func (r *PostgresRepository) EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM events WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL)`,
		eventID, userID).Scan(&exists)
	return exists, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(row rowScanner) (*Device, error) {
	var d Device
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.KeyPrefix, &d.KeyHash,
		&d.AssignedEventID, &d.RevokedAt, &d.LastUsedAt, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}
