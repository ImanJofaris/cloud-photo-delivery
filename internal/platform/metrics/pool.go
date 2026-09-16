package metrics

import "github.com/prometheus/client_golang/prometheus"

// PoolStats is the subset of pgxpool.Stat exposed as gauges.
type PoolStats interface {
	MaxConns() int32
	TotalConns() int32
	AcquiredConns() int32
	IdleConns() int32
}

// RegisterDBPool exposes connection pool saturation on /metrics.
func RegisterDBPool(reg prometheus.Registerer, stat func() PoolStats) error {
	gauges := []struct {
		name string
		help string
		get  func(PoolStats) float64
	}{
		{"db_pool_max_connections", "Configured maximum number of connections.", func(s PoolStats) float64 { return float64(s.MaxConns()) }},
		{"db_pool_total_connections", "Open connections in the pool.", func(s PoolStats) float64 { return float64(s.TotalConns()) }},
		{"db_pool_acquired_connections", "Connections currently checked out.", func(s PoolStats) float64 { return float64(s.AcquiredConns()) }},
		{"db_pool_idle_connections", "Connections currently idle.", func(s PoolStats) float64 { return float64(s.IdleConns()) }},
	}
	for _, g := range gauges {
		g := g
		if err := reg.Register(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      g.name,
			Help:      g.help,
		}, func() float64 { return g.get(stat()) })); err != nil {
			return err
		}
	}
	return nil
}
