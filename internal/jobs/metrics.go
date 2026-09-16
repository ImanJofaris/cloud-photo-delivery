package jobs

import (
	"context"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterMetrics exposes job queue depth on /metrics. Queries run at scrape
// time with a short timeout; a query error reports NaN rather than a stale
// value so the scrape shows the failure.
func RegisterMetrics(reg prometheus.Registerer, pool *pgxpool.Pool) error {
	gauges := []struct {
		name   string
		help   string
		status Status
	}{
		{"jobs_pending", "Jobs waiting to be claimed.", StatusPending},
		{"jobs_running", "Jobs currently claimed by a worker.", StatusRunning},
		{"jobs_failed", "Jobs that exhausted their attempts.", StatusFailed},
	}
	for _, g := range gauges {
		g := g
		if err := reg.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "cpd",
			Name:      g.name,
			Help:      g.help,
		}, func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var n int64
			if err := pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM jobs WHERE status = $1`, g.status).Scan(&n); err != nil {
				return math.NaN()
			}
			return float64(n)
		})); err != nil {
			return err
		}
	}
	return nil
}
