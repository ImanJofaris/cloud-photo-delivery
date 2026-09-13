package exports

import (
	"context"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Job type strings registered with the worker. Both fit the jobs.type
// VARCHAR(50) column.
const (
	JobZipGenerate   = "zip.generate"
	JobExportCleanup = "export.cleanup"
)

type GeneratePayload struct {
	ExportID uuid.UUID `json:"exportId"`
	EventID  uuid.UUID `json:"eventId"`
}

type PostgresQueue struct {
	queue *jobs.PostgresQueue
}

func NewPostgresQueue(pool *pgxpool.Pool) *PostgresQueue {
	return &PostgresQueue{queue: jobs.NewPostgresQueue(pool)}
}

func (q *PostgresQueue) EnqueueGenerate(ctx context.Context, payload GeneratePayload) error {
	_, err := q.queue.Enqueue(ctx, JobZipGenerate, payload)
	return err
}
