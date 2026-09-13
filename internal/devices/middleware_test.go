package devices

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
	"github.com/stretchr/testify/require"
)

func TestRequireDeviceOrOperator_DeviceKeyHeader(t *testing.T) {
	svc, repo, userID := newTestService(t)
	_, raw := repo.seed(userID, "Booth", nil)

	var got *Device
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, ok := FromContext(r.Context())
		require.True(t, ok)
		got = d
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", raw)
	rec := httptest.NewRecorder()
	svc.RequireDeviceOrOperator(nil)(next).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, got)
	require.Equal(t, userID, got.UserID)
}

func TestRequireDeviceOrOperator_AuthorizationDeviceScheme(t *testing.T) {
	svc, repo, userID := newTestService(t)
	_, raw := repo.seed(userID, "Booth", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := FromContext(r.Context())
		require.True(t, ok)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Device "+raw)
	rec := httptest.NewRecorder()
	svc.RequireDeviceOrOperator(nil)(next).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireDeviceOrOperator_InvalidAndRevoked(t *testing.T) {
	svc, repo, userID := newTestService(t)
	device, raw := repo.seed(userID, "Booth", nil)
	require.NoError(t, svc.Revoke(context.Background(), userID, device.ID))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := svc.RequireDeviceOrOperator(nil)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", "bogus")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "INVALID_DEVICE_KEY")

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", raw)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "DEVICE_REVOKED")
}

func TestRequireDeviceOrOperator_FallsBackToOperator(t *testing.T) {
	svc, _, _ := newTestService(t)

	operatorCalled := false
	operator := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			operatorCalled = true
			next.ServeHTTP(w, r)
		})
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer jwt-token")
	rec := httptest.NewRecorder()
	svc.RequireDeviceOrOperator(operator)(next).ServeHTTP(rec, req)

	require.True(t, operatorCalled)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireDeviceOrOperator_NoOperatorConfigured(t *testing.T) {
	svc, _, _ := newTestService(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	rec := httptest.NewRecorder()
	svc.RequireDeviceOrOperator(nil)(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "UNAUTHORIZED")
}

func TestRequireDevice_AcceptsKey(t *testing.T) {
	svc, repo, userID := newTestService(t)
	_, raw := repo.seed(userID, "Booth", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, ok := FromContext(r.Context())
		require.True(t, ok)
		require.Equal(t, userID, d.UserID)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Api-Key", raw)
	rec := httptest.NewRecorder()
	svc.RequireDevice(next).ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireDevice_RejectsOperatorToken(t *testing.T) {
	svc, _, _ := newTestService(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer jwt-token")
	rec := httptest.NewRecorder()
	svc.RequireDevice(next).ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireDeviceOrOperator_ConcurrentRequests(t *testing.T) {
	svc, repo, userID := newTestService(t)
	_, raw := repo.seed(userID, "Booth", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := FromContext(r.Context())
		require.True(t, ok)
		w.WriteHeader(http.StatusOK)
	})
	handler := svc.RequireDeviceOrOperator(nil)(next)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("X-Api-Key", raw)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
		}()
	}
	wg.Wait()
	require.Equal(t, 50, repo.touchCount())
}

func TestRateLimit_OnlyThrottlesDevices(t *testing.T) {
	svc, repo, userID := newTestService(t)
	_, raw := repo.seed(userID, "Booth", nil)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	rl := httpx.NewRateLimiter(1, 1, time.Minute)
	chain := func(h http.Handler) http.Handler {
		return svc.RequireDeviceOrOperator(nil)(RateLimit(rl)(h))
	}

	deviceReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/events/x/uploads", nil)
		req.Header.Set("X-Api-Key", raw)
		rec := httptest.NewRecorder()
		chain(next).ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, http.StatusOK, deviceReq().Code)
	require.Equal(t, http.StatusTooManyRequests, deviceReq().Code)

	operatorReq := httptest.NewRequest(http.MethodPost, "/events/x/uploads", nil)
	operatorReq.Header.Set("Authorization", "Bearer jwt")
	rec := httptest.NewRecorder()
	svc.RequireDeviceOrOperator(func(h http.Handler) http.Handler { return h })(RateLimit(rl)(next)).ServeHTTP(rec, operatorReq)
	require.Equal(t, http.StatusOK, rec.Code)
}
