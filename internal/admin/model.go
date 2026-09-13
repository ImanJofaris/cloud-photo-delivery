package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Stats struct {
	Users         int64
	Events        int64
	Photos        int64
	StorageBytes  int64
	RevenueCents  int64
	Subscriptions int64
}

type UserSummary struct {
	ID           uuid.UUID
	Email        string
	BusinessName string
	IsAdmin      bool
	StorageBytes int64
	EventCount   int64
	CreatedAt    time.Time
}

type SubscriptionSummary struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	UserEmail         string
	PlanID            string
	Status            string
	Interval          string
	CurrentPeriodEnd  *time.Time
	CancelAtPeriodEnd bool
	CreatedAt         time.Time
}

type QueueHealth struct {
	Pending         int64
	Running         int64
	Failed          int64
	OldestPendingAt *time.Time
}

type Health struct {
	Status    string
	Queue     QueueHealth
	CheckedAt time.Time
}

func (h Health) QueueDepth() int64 {
	return h.Queue.Pending + h.Queue.Running
}

type StorageTotals struct {
	StorageBytes int64
	Photos       int64
}

type ReconcileReport struct {
	CheckedAt      time.Time
	DBStorageBytes int64
	DBPhotos       int64
	R2StorageBytes int64
	R2Objects      int64
}

func (r ReconcileReport) DriftBytes() int64 {
	return r.R2StorageBytes - r.DBStorageBytes
}

func (r ReconcileReport) DriftObjects() int64 {
	return r.R2Objects - r.DBPhotos
}

func (r ReconcileReport) InSync() bool {
	return r.DriftBytes() == 0 && r.DriftObjects() == 0
}

type Repository interface {
	Stats(ctx context.Context) (*Stats, error)
	ListUsers(ctx context.Context, cursor *Cursor, limit int) ([]*UserSummary, error)
	ListSubscriptions(ctx context.Context, cursor *Cursor, limit int) ([]*SubscriptionSummary, error)
	QueueHealth(ctx context.Context) (*QueueHealth, error)
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	StorageTotals(ctx context.Context) (*StorageTotals, error)
}
