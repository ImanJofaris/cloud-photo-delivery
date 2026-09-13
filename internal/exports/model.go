package exports

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
	StatusExpired    Status = "expired"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusProcessing, StatusReady, StatusFailed, StatusExpired:
		return true
	default:
		return false
	}
}

// Active reports whether an export is queued or in flight and should be reused
// by a duplicate request instead of generating a second archive.
func (s Status) Active() bool {
	return s == StatusPending || s == StatusProcessing
}

type Export struct {
	ID           uuid.UUID
	EventID      uuid.UUID
	Status       Status
	ObjectKey    *string
	FileSize     *int64
	ErrorMessage *string
	ExpiresAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
