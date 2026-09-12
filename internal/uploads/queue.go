package uploads

import (
	"context"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/jackc/pgx/v5/pgxpool"
)

// JobProcessPhoto is the job type enqueued when an upload completes. The
// worker registers this type alongside image.process.
const JobProcessPhoto = "PROCESS_PHOTO"

type PostgresQueue struct {
	queue *jobs.PostgresQueue
}

func NewPostgresQueue(pool *pgxpool.Pool) *PostgresQueue {
	return &PostgresQueue{queue: jobs.NewPostgresQueue(pool)}
}

func (q *PostgresQueue) EnqueueProcessPhoto(ctx context.Context, photoID, eventID uuid.UUID) error {
	_, err := q.queue.Enqueue(ctx, JobProcessPhoto, photos.ProcessPayload{
		PhotoID: photoID,
		EventID: eventID,
	})
	return err
}
