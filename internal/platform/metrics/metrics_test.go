package metrics

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

func TestObserveHTTP_CountsByRouteAndStatus(t *testing.T) {
	m := New()
	m.ObserveHTTP(http.MethodGet, "/api/v1/events/{eventID}", http.StatusOK, 5*time.Millisecond)
	m.ObserveHTTP(http.MethodGet, "/api/v1/events/{eventID}", http.StatusOK, 5*time.Millisecond)
	m.ObserveHTTP(http.MethodGet, "/api/v1/events/{eventID}", http.StatusNotFound, time.Millisecond)

	require.Equal(t, float64(2),
		testutil.ToFloat64(m.requestsTotal.WithLabelValues(http.MethodGet, "/api/v1/events/{eventID}", "200")))
	require.Equal(t, float64(1),
		testutil.ToFloat64(m.requestsTotal.WithLabelValues(http.MethodGet, "/api/v1/events/{eventID}", "404")))
}

func TestInFlight(t *testing.T) {
	m := New()
	require.Equal(t, float64(0), testutil.ToFloat64(m.inFlight))
	m.IncInFlight()
	m.IncInFlight()
	require.Equal(t, float64(2), testutil.ToFloat64(m.inFlight))
	m.DecInFlight()
	require.Equal(t, float64(1), testutil.ToFloat64(m.inFlight))
}

func TestRecordUploadAndJob(t *testing.T) {
	m := New()
	m.RecordUpload(OutcomeNameInitialized)
	m.RecordUpload(OutcomeNameInitialized)
	m.RecordUpload(OutcomeNameFailed)
	m.RecordJob("PROCESS_PHOTO", "done", 250*time.Millisecond)
	m.RecordJob("PROCESS_PHOTO", "retry", 500*time.Millisecond)

	require.Equal(t, float64(2), testutil.ToFloat64(m.uploadsTotal.WithLabelValues(OutcomeNameInitialized)))
	require.Equal(t, float64(1), testutil.ToFloat64(m.uploadsTotal.WithLabelValues(OutcomeNameFailed)))
	require.Equal(t, float64(1), testutil.ToFloat64(m.jobsTotal.WithLabelValues("PROCESS_PHOTO", "done")))
	require.Equal(t, float64(1), testutil.ToFloat64(m.jobsTotal.WithLabelValues("PROCESS_PHOTO", "retry")))
}

const (
	OutcomeNameInitialized = "initialized"
	OutcomeNameFailed      = "failed"
)

func TestHandler_ServesMetrics(t *testing.T) {
	m := New()
	m.ObserveHTTP(http.MethodGet, "/healthz", http.StatusOK, time.Millisecond)

	srv := httptest.NewServer(m.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/metrics")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "cpd_http_requests_total")
	require.Contains(t, string(body), `route="/healthz"`)
}

func TestRegisterDBPool(t *testing.T) {
	m := New()
	stats := fakePoolStats{max: 20, total: 5, acquired: 3, idle: 2}
	require.NoError(t, RegisterDBPool(m.Registry(), func() PoolStats { return stats }))

	families, err := m.Registry().Gather()
	require.NoError(t, err)

	values := map[string]float64{}
	for _, f := range families {
		if strings.HasPrefix(f.GetName(), "cpd_db_pool_") {
			values[f.GetName()] = f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	require.Equal(t, float64(20), values["cpd_db_pool_max_connections"])
	require.Equal(t, float64(5), values["cpd_db_pool_total_connections"])
	require.Equal(t, float64(3), values["cpd_db_pool_acquired_connections"])
	require.Equal(t, float64(2), values["cpd_db_pool_idle_connections"])
}

type fakePoolStats struct{ max, total, acquired, idle int32 }

func (f fakePoolStats) MaxConns() int32      { return f.max }
func (f fakePoolStats) TotalConns() int32    { return f.total }
func (f fakePoolStats) AcquiredConns() int32 { return f.acquired }
func (f fakePoolStats) IdleConns() int32     { return f.idle }

type fakeStore struct {
	fail bool
}

func (f *fakeStore) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return f.result("url")
}
func (f *fakeStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return f.result("url")
}
func (f *fakeStore) Head(context.Context, string) (int64, error) {
	if f.fail {
		return 0, r2.ErrNotFound
	}
	return 1, nil
}
func (f *fakeStore) Delete(context.Context, string) error { return f.err() }
func (f *fakeStore) Put(context.Context, string, string, []byte) error {
	return f.err()
}
func (f *fakeStore) Get(context.Context, string) ([]byte, error) {
	if f.fail {
		return nil, r2.ErrNotFound
	}
	return []byte("x"), nil
}
func (f *fakeStore) GetReader(context.Context, string) (io.ReadCloser, error) {
	if f.fail {
		return nil, r2.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader([]byte("x"))), nil
}
func (f *fakeStore) PutReader(context.Context, string, string, io.Reader, int64) error {
	return f.err()
}
func (f *fakeStore) Bucket() string { return "bucket" }
func (f *fakeStore) CreateMultipartUpload(context.Context, string, string) (string, error) {
	return f.result("upload-id")
}
func (f *fakeStore) PresignUploadPart(context.Context, string, string, int, time.Duration) (string, error) {
	return f.result("url")
}
func (f *fakeStore) CompleteMultipartUpload(context.Context, string, string, []r2.CompletePart) error {
	return f.err()
}
func (f *fakeStore) AbortMultipartUpload(context.Context, string, string) error { return f.err() }

func (f *fakeStore) result(v string) (string, error) {
	if f.fail {
		return "", r2.ErrNotFound
	}
	return v, nil
}

func (f *fakeStore) err() error {
	if f.fail {
		return r2.ErrNotFound
	}
	return nil
}

func TestWrapStore_CountsOperationsAndErrors(t *testing.T) {
	m := New()

	store := WrapStore(&fakeStore{}, m)
	_, err := store.Head(context.Background(), "key")
	require.NoError(t, err)
	_, err = store.Get(context.Background(), "key")
	require.NoError(t, err)

	require.Equal(t, float64(1), testutil.ToFloat64(m.r2RequestsTotal.WithLabelValues("head")))
	require.Equal(t, float64(0), testutil.ToFloat64(m.r2ErrorsTotal.WithLabelValues("head")))

	failing := WrapStore(&fakeStore{fail: true}, m)
	_, err = failing.Get(context.Background(), "key")
	require.Error(t, err)

	require.Equal(t, float64(2), testutil.ToFloat64(m.r2RequestsTotal.WithLabelValues("get")))
	require.Equal(t, float64(1), testutil.ToFloat64(m.r2ErrorsTotal.WithLabelValues("get")))
}

func TestWrapStore_NilReturnsInput(t *testing.T) {
	require.Nil(t, WrapStore(nil, New()))

	store := &fakeStore{}
	require.Same(t, store, WrapStore(store, nil))
}

func TestWrapStore_BucketPassthrough(t *testing.T) {
	store := WrapStore(&fakeStore{}, New())
	require.Equal(t, "bucket", store.Bucket())
}
