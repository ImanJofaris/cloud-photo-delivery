package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending Status = "pending"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Job struct {
	ID          uuid.UUID
	Type        string
	Payload     json.RawMessage
	Status      Status
	Attempts    int
	MaxAttempts int
	LastError   *string
	RunAt       time.Time
	LockedAt    *time.Time
	LockedBy    *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

var (
	ErrNoJob = errors.New("no job available")
)

type Queue interface {
	// Claim atomically locks the next ready job for workerID, marking it
	// running and returning it. Returns ErrNoJob when none are ready.
	Claim(ctx context.Context, workerID string) (*Job, error)
	// Complete marks a running job done.
	Complete(ctx context.Context, id uuid.UUID) error
	// Fail records an attempt failure. It either schedules a retry with
	// backoff (returning retry=true) or marks the job failed permanently.
	Fail(ctx context.Context, id uuid.UUID, cause string) (retry bool, err error)
	// Release returns a running job to pending without counting an attempt.
	// Used on graceful shutdown so another worker can pick it up immediately.
	Release(ctx context.Context, id uuid.UUID) error
}
