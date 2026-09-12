package uploads

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("upload not found")
)

type IdempotencyRecord struct {
	UserID      uuid.UUID
	PhotoID     uuid.UUID
	RequestHash string
}

type Repository interface {
	EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
	LookupIdempotency(ctx context.Context, key string) (*IdempotencyRecord, error)
	SaveIdempotency(ctx context.Context, key string, rec IdempotencyRecord) error
	PartETags(ctx context.Context, photoID uuid.UUID) (map[int]string, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM events WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL)`,
		eventID, userID).Scan(&exists)
	return exists, err
}

func (r *PostgresRepository) LookupIdempotency(ctx context.Context, key string) (*IdempotencyRecord, error) {
	var rec IdempotencyRecord
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, photo_id, request_hash FROM upload_idempotency WHERE idempotency_key = $1`, key).
		Scan(&rec.UserID, &rec.PhotoID, &rec.RequestHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &rec, nil
}

func (r *PostgresRepository) SaveIdempotency(ctx context.Context, key string, rec IdempotencyRecord) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO upload_idempotency (idempotency_key, user_id, photo_id, request_hash)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (idempotency_key) DO NOTHING`,
		key, rec.UserID, rec.PhotoID, rec.RequestHash)
	return err
}

func (r *PostgresRepository) PartETags(ctx context.Context, photoID uuid.UUID) (map[int]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT part_number, COALESCE(etag, '') FROM upload_parts WHERE photo_id = $1`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int]string{}
	for rows.Next() {
		var n int
		var etag string
		if err := rows.Scan(&n, &etag); err != nil {
			return nil, err
		}
		out[n] = etag
	}
	return out, rows.Err()
}
