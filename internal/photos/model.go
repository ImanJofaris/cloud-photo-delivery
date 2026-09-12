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
