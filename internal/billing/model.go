package billing

import (
	"time"

	"github.com/google/uuid"
)

const FreePlanID = "free"

type Status string

const (
	StatusTrialing Status = "trialing"
	StatusActive   Status = "active"
	StatusPastDue  Status = "past_due"
	StatusCanceled Status = "canceled"
	StatusExpired  Status = "expired"
)

func (s Status) Valid() bool {
	switch s {
	case StatusTrialing, StatusActive, StatusPastDue, StatusCanceled, StatusExpired:
		return true
	}
	return false
}

func (s Status) Terminal() bool {
	return s == StatusCanceled || s == StatusExpired
}

func (s Status) Usable() bool {
	return !s.Terminal()
}

type PlanLimits struct {
	Events            int   `json:"events"`
	PhotosPerEvent    int   `json:"photosPerEvent"`
	StorageBytes      int64 `json:"storageBytes"`
	RetentionDays     int   `json:"retentionDays"`
	APIAccess         bool  `json:"apiAccess"`
	Branding          bool  `json:"branding"`
	OriginalDownloads bool  `json:"originalDownloads"`
}

type Plan struct {
	ID         string
	Name       string
	PriceCents int
	Currency   string
	Interval   string
	Limits     PlanLimits
	Active     bool
}

type Subscription struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	PlanID             string
	Status             Status
	Provider           string
	ProviderRef        *string
	Interval           string
	CurrentPeriodStart *time.Time
	CurrentPeriodEnd   *time.Time
	CancelAtPeriodEnd  bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type InvoiceStatus string

const (
	InvoiceOpen InvoiceStatus = "open"
	InvoicePaid InvoiceStatus = "paid"
	InvoiceVoid InvoiceStatus = "void"
)

type Invoice struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	SubscriptionID *uuid.UUID
	AmountCents    int
	Currency       string
	Status         InvoiceStatus
	ProviderRef    *string
	IssuedAt       time.Time
	PaidAt         *time.Time
}

type Usage struct {
	Events       int
	StorageBytes int64
}

type SubscriptionView struct {
	Subscription *Subscription
	Plan         *Plan
	Usage        Usage
}
