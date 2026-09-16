package metrics

import (
	"context"
	"io"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

// WrapStore decorates an ObjectStore so every operation reports latency and
// errors to Prometheus. Failures are counted, never swallowed: the wrapped
// store returns the original error untouched.
func WrapStore(store r2.ObjectStore, m *Metrics) r2.ObjectStore {
	if store == nil || m == nil {
		return store
	}
	return &storeObserver{next: store, m: m}
}

type storeObserver struct {
	next r2.ObjectStore
	m    *Metrics
}

func (o *storeObserver) observe(op string, start time.Time, err error) {
	o.m.observeR2(op, time.Since(start), err)
}

func (o *storeObserver) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	start := time.Now()
	v, err := o.next.PresignPut(ctx, key, contentType, ttl)
	o.observe("presign_put", start, err)
	return v, err
}

func (o *storeObserver) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	start := time.Now()
	v, err := o.next.PresignGet(ctx, key, ttl)
	o.observe("presign_get", start, err)
	return v, err
}

func (o *storeObserver) Head(ctx context.Context, key string) (int64, error) {
	start := time.Now()
	v, err := o.next.Head(ctx, key)
	o.observe("head", start, err)
	return v, err
}

func (o *storeObserver) Delete(ctx context.Context, key string) error {
	start := time.Now()
	err := o.next.Delete(ctx, key)
	o.observe("delete", start, err)
	return err
}

func (o *storeObserver) Put(ctx context.Context, key, contentType string, body []byte) error {
	start := time.Now()
	err := o.next.Put(ctx, key, contentType, body)
	o.observe("put", start, err)
	return err
}

func (o *storeObserver) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()
	v, err := o.next.Get(ctx, key)
	o.observe("get", start, err)
	return v, err
}

func (o *storeObserver) GetReader(ctx context.Context, key string) (io.ReadCloser, error) {
	start := time.Now()
	v, err := o.next.GetReader(ctx, key)
	o.observe("get_reader", start, err)
	return v, err
}

func (o *storeObserver) PutReader(ctx context.Context, key, contentType string, r io.Reader, size int64) error {
	start := time.Now()
	err := o.next.PutReader(ctx, key, contentType, r, size)
	o.observe("put_reader", start, err)
	return err
}

func (o *storeObserver) Bucket() string { return o.next.Bucket() }

func (o *storeObserver) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	start := time.Now()
	v, err := o.next.CreateMultipartUpload(ctx, key, contentType)
	o.observe("create_multipart", start, err)
	return v, err
}

func (o *storeObserver) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int, ttl time.Duration) (string, error) {
	start := time.Now()
	v, err := o.next.PresignUploadPart(ctx, key, uploadID, partNumber, ttl)
	o.observe("presign_part", start, err)
	return v, err
}

func (o *storeObserver) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []r2.CompletePart) error {
	start := time.Now()
	err := o.next.CompleteMultipartUpload(ctx, key, uploadID, parts)
	o.observe("complete_multipart", start, err)
	return err
}

func (o *storeObserver) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	start := time.Now()
	err := o.next.AbortMultipartUpload(ctx, key, uploadID)
	o.observe("abort_multipart", start, err)
	return err
}
