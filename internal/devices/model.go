package devices

import (
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Name            string
	KeyPrefix       string
	KeyHash         string
	AssignedEventID *uuid.UUID
	RevokedAt       *time.Time
	LastUsedAt      *time.Time
	CreatedAt       time.Time
}

func (d *Device) Revoked() bool { return d.RevokedAt != nil }
