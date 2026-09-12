package events

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusUpcoming  Status = "upcoming"
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusArchived  Status = "archived"
)

type Visibility string

const (
	VisibilityPublic   Visibility = "public"
	VisibilityPassword Visibility = "password"
	VisibilityPrivate  Visibility = "private"
)

type Event struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	Slug         string
	ClientName   string
	ClientEmail  string
	Location     string
	Description  string
	EventDate    *time.Time
	Status       Status
	CoverPhotoID *uuid.UUID
	StorageBytes int64
	PhotoCount   int64
	GuestCount   int64
	ExpiresAt    *time.Time
	DeletedAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Settings struct {
	EventID               uuid.UUID
	Visibility            Visibility
	PasswordHash          string
	AllowDownload         bool
	AllowOriginalDownload bool
	WatermarkEnabled      bool
	PasswordChangedAt     *time.Time
	UpdatedAt             time.Time
}

func (s Status) Valid() bool {
	switch s {
	case StatusUpcoming, StatusActive, StatusCompleted, StatusArchived:
		return true
	default:
		return false
	}
}

var allowedTransitions = map[Status][]Status{
	StatusUpcoming:  {StatusActive, StatusArchived},
	StatusActive:    {StatusCompleted, StatusArchived},
	StatusCompleted: {StatusArchived},
	StatusArchived:  {},
}

func CanTransition(from, to Status) bool {
	for _, s := range allowedTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// CanSetVisibility reports whether downloads are permitted for the visibility.
func (s Settings) CanSetVisibility() bool {
	return s.Visibility != VisibilityPrivate || !s.AllowDownload
}

func (v Visibility) Valid() bool {
	switch v {
	case VisibilityPublic, VisibilityPassword, VisibilityPrivate:
		return true
	default:
		return false
	}
}
