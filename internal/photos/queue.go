package photos

import (
	"context"

	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobObjectCleanup is the worker job type that removes stored objects. It
// matches the type registered in cmd/worker.
const JobObjectCleanup = "object.cleanup"

type PostgresQueue struct {
	queue *jobs.PostgresQueue
}

func NewPostgresQueue(pool *pgxpool.Pool) *PostgresQueue {
	return &PostgresQueue{queue: jobs.NewPostgresQueue(pool)}
}

func (q *PostgresQueue) EnqueueObjectCleanup(ctx context.Context, payload CleanupPayload) error {
	_, err := q.queue.Enqueue(ctx, JobObjectCleanup, payload)
	return err
}
