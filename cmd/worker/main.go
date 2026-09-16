package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/admin"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/exports"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/metrics"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/httpx"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/imaging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

// Job type constants. The upload path enqueues PROCESS_PHOTO; the phase doc
// names it image.process. Both are registered to the same handler so jobs
// created by either convention are processed.
const (
	jobProcessPhoto = "PROCESS_PHOTO"
	jobImageProcess = "image.process"
)

// lifecycleInterval is how often the scheduler enqueues expiry and purge
// scans. EnqueueUnique keeps at most one instance in flight.
const lifecycleInterval = 15 * time.Minute

// reconcileInterval is how often storage counters are compared with R2.
const reconcileInterval = 24 * time.Hour

func main() {
	cfg, err := config.Load()
	if err != nil {
		logging.New("dev", "info").Error("config load failed", "error", err)
		os.Exit(1)
	}

	log := logging.New(cfg.Env, cfg.LogLevel)

	if err := cfg.ValidateStorage(); err != nil {
		log.Error("storage config invalid", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.NewWithOptions(ctx, cfg.DatabaseURL, cfg.DatabaseOptions(log))
	if err != nil {
		log.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	m := metrics.New()
	if err := metrics.RegisterDBPool(m.Registry(), func() metrics.PoolStats { return pool.Stat() }); err != nil {
		log.Error("metrics registration failed", "error", err)
		os.Exit(1)
	}
	if err := jobs.RegisterMetrics(m.Registry(), pool.Pool); err != nil {
		log.Error("queue metrics registration failed", "error", err)
		os.Exit(1)
	}

	store, err := r2.New(ctx, r2.Options{
		Endpoint:  cfg.R2Endpoint,
		AccessKey: cfg.R2AccessKey,
		SecretKey: cfg.R2SecretKey,
		Bucket:    cfg.R2Bucket,
		Region:    cfg.R2Region,
	})
	if err != nil {
		log.Error("object store init failed", "error", err)
		os.Exit(1)
	}
	var instrumentedStore r2.ObjectStore = metrics.WrapStore(store, m)

	photoRepo := photos.NewRepository(pool.Pool)
	eventRepo := events.NewRepository(pool.Pool)
	exportRepo := exports.NewRepository(pool.Pool)
	std := imaging.New()
	processor := photos.NewProcessor(photoRepo, instrumentedStore, std, std, std, log, photoRepo.EventOwner)

	registry := jobs.NewRegistry().
		Register(jobProcessPhoto, processor.Handle).
		Register(jobImageProcess, processor.Handle)

	// The cleanup handler removes an orphaned original when processing fails
	// permanently. It is idempotent and tolerant of a missing object.
	registry.Register("object.cleanup", photos.CleanupHandler(photoRepo, instrumentedStore, log))

	// Lifecycle jobs run on a worker-side ticker: expire marks events whose
	// expires_at passed and warns owners; purge removes events past the grace
	// period from R2 and the database.
	registry.Register(events.JobEventExpire, events.ExpireHandler(
		eventRepo, events.LogNotifier{Log: log}, time.Duration(cfg.ExpiryWarnDays)*24*time.Hour, time.Now, log))
	registry.Register(events.JobEventPurge, events.PurgeHandler(
		eventRepo, instrumentedStore, time.Duration(cfg.EventPurgeGraceDays)*24*time.Hour, time.Now, log))

	// Bulk ZIP exports: generation streams originals from storage into a
	// temp-file archive; cleanup expires archives and fails stuck exports.
	registry.Register(exports.JobZipGenerate, exports.GenerateHandler(
		exportRepo, photoRepo.EventOwner, photoRepo, instrumentedStore, cfg.ExportTTL, log))
	registry.Register(exports.JobExportCleanup, exports.CleanupHandler(exportRepo, instrumentedStore, log))

	// Storage reconciliation compares the tenant storage counters with the
	// original objects in R2 and reports drift; it never deletes objects.
	// Reconciliation needs ListObjects, which only the concrete S3 store
	// exposes; it is not part of r2.ObjectStore and is not metric-wrapped.
	registry.Register(admin.JobStorageReconcile, admin.ReconcileHandler(
		admin.NewRepository(pool.Pool), store, time.Now, admin.LogReconcileReporter{Log: log}))

	queue := jobs.NewPostgresQueue(pool.Pool)
	workerID := workerName(cfg.Env)

	scheduler := jobs.NewScheduler(queue, log,
		jobs.Schedule{Type: events.JobEventExpire, Every: lifecycleInterval},
		jobs.Schedule{Type: events.JobEventPurge, Every: lifecycleInterval},
		jobs.Schedule{Type: exports.JobExportCleanup, Every: time.Hour},
		jobs.Schedule{Type: admin.JobStorageReconcile, Every: reconcileInterval},
	)
	go scheduler.Run(ctx)

	poller := jobs.NewPoller(queue, registry, log, workerID,
		jobs.WithConcurrency(cfg.WorkerConcurrency),
		jobs.WithPollInterval(cfg.WorkerPollEvery),
		jobs.WithObserver(m),
	)

	metricsSrv := httpx.NewInternalServer(cfg.WorkerMetricsAddr, m.Handler())
	if cfg.WorkerMetricsAddr != "" {
		go func() {
			log.Info("metrics server listening", "addr", cfg.WorkerMetricsAddr)
			if err := metricsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("metrics server error", "error", err)
			}
		}()
	}

	log.Info("worker started", "env", cfg.Env, "types", registry.Types())
	poller.Run(ctx)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("metrics shutdown failed", "error", err)
	}
	log.Info("worker shutting down")
}

func workerName(env string) string {
	host, _ := os.Hostname()
	return env + "/" + host + "/" + uuid.NewString()[:8]
}
