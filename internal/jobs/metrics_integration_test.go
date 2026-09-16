//go:build integration

package jobs_test

import (
	"context"
	"strings"
	"testing"

	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func queueGauges(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	require.NoError(t, err)

	out := map[string]float64{}
	for _, f := range families {
		if strings.HasPrefix(f.GetName(), "cpd_jobs_") {
			out[f.GetName()] = f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	return out
}

func TestRegisterMetrics_ReportsQueueDepth(t *testing.T) {
	pool, q := setupQueue(t)
	ctx := context.Background()

	reg := prometheus.NewRegistry()
	require.NoError(t, jobs.RegisterMetrics(reg, pool))

	// One partial index-friendly baseline: empty queue.
	empty := queueGauges(t, reg)
	require.Equal(t, float64(0), empty["cpd_jobs_pending"])
	require.Equal(t, float64(0), empty["cpd_jobs_running"])
	require.Equal(t, float64(0), empty["cpd_jobs_failed"])

	_, err := q.Enqueue(ctx, "test.job", map[string]string{"n": "1"})
	require.NoError(t, err)
	_, err = q.Enqueue(ctx, "test.job", map[string]string{"n": "2"})
	require.NoError(t, err)

	_, err = q.Claim(ctx, "worker-1")
	require.NoError(t, err)

	gauges := queueGauges(t, reg)
	require.Equal(t, float64(1), gauges["cpd_jobs_pending"])
	require.Equal(t, float64(1), gauges["cpd_jobs_running"])
	require.Equal(t, float64(0), gauges["cpd_jobs_failed"])
}
