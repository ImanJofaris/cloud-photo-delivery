package uploads

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresQueue struct {
	pool *pgxpool.Pool
}

func NewPostgresQueue(pool *pgxpool.Pool) *PostgresQueue {
	return &PostgresQueue{pool: pool}
}

func (q *PostgresQueue) EnqueueProcessPhoto(ctx context.Context, photoID, eventID uuid.UUID) error {
	payload, err := json.Marshal(map[string]string{
		"photoId": photoID.String(),
		"eventId": eventID.String(),
	})
	if err != nil {
		return err
	}
	_, err = q.pool.Exec(ctx,
		`INSERT INTO jobs (type, payload) VALUES ('PROCESS_PHOTO', $1)`, payload)
	return err
}
