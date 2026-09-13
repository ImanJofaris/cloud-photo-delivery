package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Counters holds the recorded totals for an event (all time) or a single day.
type Counters struct {
	GalleryViews   int64
	UniqueVisitors int64
	Downloads      int64
	QRScans        int64
}

type DayCounters struct {
	Day time.Time
	Counters
}

// EventReport is the event analytics endpoint payload before mapping.
type EventReport struct {
	EventID    uuid.UUID
	PhotoCount int64
	Totals     Counters
	Daily      []DayCounters
}

// AccountReport aggregates every event owned by one operator.
type AccountReport struct {
	EventCount int64
	PhotoCount int64
	Totals     Counters
	Daily      []DayCounters
}

// Repository owns all analytics SQL. Counter writes are upserts so concurrent
// gallery hits never race on a missing row.
type Repository interface {
	RecordView(ctx context.Context, eventID uuid.UUID, day time.Time, visitorHash string, qrScan bool) error
	RecordDownload(ctx context.Context, eventID uuid.UUID, day time.Time) error
	EventSummary(ctx context.Context, eventID uuid.UUID) (EventReport, error)
	EventDaily(ctx context.Context, eventID uuid.UUID, since time.Time) ([]DayCounters, error)
	AccountSummary(ctx context.Context, userID uuid.UUID) (AccountReport, error)
	AccountDaily(ctx context.Context, userID uuid.UUID, since time.Time) ([]DayCounters, error)
}

// EventChecker is implemented by the photos repository and proves ownership of
// a non-deleted event.
type EventChecker interface {
	EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
}
