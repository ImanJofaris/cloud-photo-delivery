package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID               uuid.UUID
	Email            string
	PasswordHash     string
	BusinessName     string
	EmailVerifiedAt  *time.Time
	FailedLoginCount int
	LockedUntil      *time.Time
	IsAdmin          bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}
