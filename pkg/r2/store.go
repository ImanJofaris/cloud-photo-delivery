package r2

import (
	"context"
	"io"
	"time"
)

type CompletePart struct {
	PartNumber int
	ETag       string
}

type ObjectStore interface {
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Head(ctx context.Context, key string) (int64, error)
	Delete(ctx context.Context, key string) error
	Put(ctx context.Context, key, contentType string, body []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	GetReader(ctx context.Context, key string) (io.ReadCloser, error)
	PutReader(ctx context.Context, key, contentType string, r io.Reader, size int64) error
	Bucket() string

	CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error)
	PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int, ttl time.Duration) (string, error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletePart) error
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}
