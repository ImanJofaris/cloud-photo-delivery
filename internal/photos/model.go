package photos

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusUploading  Status = "UPLOADING"
	StatusProcessing Status = "PROCESSING"
	StatusReady      Status = "READY"
	StatusFailed     Status = "FAILED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusUploading, StatusProcessing, StatusReady, StatusFailed:
		return true
	default:
		return false
	}
}

type UploadKind string

const (
	KindSimple    UploadKind = "simple"
	KindMultipart UploadKind = "multipart"
)

type Photo struct {
	ID                uuid.UUID
	EventID           uuid.UUID
	StorageKey        string
	ThumbnailKey      *string
	OptimizedKey      *string
	MediumKey         *string
	OriginalFilename  *string
	MimeType          string
	FileSize          int64
	Width             *int
	Height            *int
	Status            Status
	UploadKind        UploadKind
	MultipartUploadID *string
	ErrorMessage      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Actor is the authenticated caller for owner-facing photo operations: an
// operator (JWT) or a device key limited to one assigned event.
type Actor struct {
	UserID        uuid.UUID
	DeviceID      uuid.UUID
	AssignedEvent *uuid.UUID
}

func (a Actor) IsDevice() bool { return a.DeviceID != uuid.Nil }

// VariantNames lists the derivative variants that exist for a photo.
func VariantNames(p *Photo) []string {
	out := make([]string, 0, 3)
	if p.ThumbnailKey != nil && *p.ThumbnailKey != "" {
		out = append(out, "thumbnail")
	}
	if p.MediumKey != nil && *p.MediumKey != "" {
		out = append(out, "medium")
	}
	if p.OptimizedKey != nil && *p.OptimizedKey != "" {
		out = append(out, "large")
	}
	return out
}
