package r2

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrUnavailable is returned when no object store is configured (for example a
// test process that builds the router without R2 credentials).
var ErrUnavailable = errors.New("object storage is not configured")

// UnavailableStore is a fail-closed ObjectStore used when storage is not
// configured. It lets the router mount upload routes while guaranteeing that
// no signed URL is ever issued without real credentials.
type UnavailableStore struct{}

func (UnavailableStore) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return "", ErrUnavailable
}

func (UnavailableStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", ErrUnavailable
}

func (UnavailableStore) Head(context.Context, string) (int64, error) { return 0, ErrUnavailable }

func (UnavailableStore) Delete(context.Context, string) error { return ErrUnavailable }

func (UnavailableStore) Put(context.Context, string, string, []byte) error { return ErrUnavailable }

func (UnavailableStore) Get(context.Context, string) ([]byte, error) { return nil, ErrUnavailable }

func (UnavailableStore) GetReader(context.Context, string) (io.ReadCloser, error) {
	return nil, ErrUnavailable
}

func (UnavailableStore) PutReader(context.Context, string, string, io.Reader, int64) error {
	return ErrUnavailable
}

func (UnavailableStore) Bucket() string { return "" }

func (UnavailableStore) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return "", ErrUnavailable
}

func (UnavailableStore) PresignUploadPart(context.Context, string, string, int, time.Duration) (string, error) {
	return "", ErrUnavailable
}

func (UnavailableStore) CompleteMultipartUpload(context.Context, string, string, []CompletePart) error {
	return ErrUnavailable
}

func (UnavailableStore) AbortMultipartUpload(context.Context, string, string) error {
	return ErrUnavailable
}
