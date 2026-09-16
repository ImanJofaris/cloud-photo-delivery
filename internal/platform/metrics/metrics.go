package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "cpd"

// Metrics owns the Prometheus collectors for one process. Each process gets
// its own registry; /metrics is served from a dedicated internal listener.
type Metrics struct {
	reg *prometheus.Registry

	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	inFlight        prometheus.Gauge

	uploadsTotal *prometheus.CounterVec

	r2RequestsTotal *prometheus.CounterVec
	r2ErrorsTotal   *prometheus.CounterVec
	r2Duration      *prometheus.HistogramVec

	jobsTotal   *prometheus.CounterVec
	jobDuration *prometheus.HistogramVec
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		requestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, route pattern and status.",
		}, []string{"method", "route", "status"}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method and route pattern.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "route"}),
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_in_flight_requests",
			Help:      "HTTP requests currently being served.",
		}),
		uploadsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "uploads_total",
			Help:      "Upload operations by outcome (initialized, completed, failed).",
		}, []string{"outcome"}),
		r2RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "r2_requests_total",
			Help:      "Object storage operations by operation.",
		}, []string{"operation"}),
		r2ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "r2_errors_total",
			Help:      "Object storage errors by operation.",
		}, []string{"operation"}),
		r2Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "r2_request_duration_seconds",
			Help:      "Object storage latency by operation.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"operation"}),
		jobsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "worker_jobs_total",
			Help:      "Worker job completions by type and outcome.",
		}, []string{"type", "outcome"}),
		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "worker_job_duration_seconds",
			Help:      "Worker job duration by type.",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
		}, []string{"type"}),
	}

	reg.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.inFlight,
		m.uploadsTotal,
		m.r2RequestsTotal,
		m.r2ErrorsTotal,
		m.r2Duration,
		m.jobsTotal,
		m.jobDuration,
	)
	return m
}

func (m *Metrics) Registry() *prometheus.Registry { return m.reg }

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveHTTP(method, route string, status int, d time.Duration) {
	m.requestsTotal.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.requestDuration.WithLabelValues(method, route).Observe(d.Seconds())
}

func (m *Metrics) IncInFlight() { m.inFlight.Inc() }
func (m *Metrics) DecInFlight() { m.inFlight.Dec() }

func (m *Metrics) RecordUpload(outcome string) {
	m.uploadsTotal.WithLabelValues(outcome).Inc()
}

func (m *Metrics) RecordJob(jobType, outcome string, d time.Duration) {
	m.jobsTotal.WithLabelValues(jobType, outcome).Inc()
	m.jobDuration.WithLabelValues(jobType).Observe(d.Seconds())
}

func (m *Metrics) observeR2(operation string, d time.Duration, err error) {
	m.r2RequestsTotal.WithLabelValues(operation).Inc()
	m.r2Duration.WithLabelValues(operation).Observe(d.Seconds())
	if err != nil {
		m.r2ErrorsTotal.WithLabelValues(operation).Inc()
	}
}
