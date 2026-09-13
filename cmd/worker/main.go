package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/jobs"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/config"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/logging"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/database"
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

	pool, err := database.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

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

	photoRepo := photos.NewRepository(pool.Pool)
	eventRepo := events.NewRepository(pool.Pool)
	std := imaging.New()
	processor := photos.NewProcessor(photoRepo, store, std, std, std, log, photoRepo.EventOwner)

	registry := jobs.NewRegistry().
		Register(jobProcessPhoto, processor.Handle).
		Register(jobImageProcess, processor.Handle)

	// The cleanup handler removes an orphaned original when processing fails
	// permanently. It is idempotent and tolerant of a missing object.
	registry.Register("object.cleanup", photos.CleanupHandler(photoRepo, store, log))

	// Lifecycle jobs run on a worker-side ticker: expire marks events whose
	// expires_at passed and warns owners; purge removes events past the grace
	// period from R2 and the database.
	registry.Register(events.JobEventExpire, events.ExpireHandler(
		eventRepo, events.LogNotifier{Log: log}, time.Duration(cfg.ExpiryWarnDays)*24*time.Hour, time.Now, log))
	registry.Register(events.JobEventPurge, events.PurgeHandler(
		eventRepo, store, time.Duration(cfg.EventPurgeGraceDays)*24*time.Hour, time.Now, log))

	queue := jobs.NewPostgresQueue(pool.Pool)
	workerID := workerName(cfg.Env)

	scheduler := jobs.NewScheduler(queue, log,
		jobs.Schedule{Type: events.JobEventExpire, Every: lifecycleInterval},
		jobs.Schedule{Type: events.JobEventPurge, Every: lifecycleInterval},
	)
	go scheduler.Run(ctx)

	poller := jobs.NewPoller(queue, registry, log, workerID,
		jobs.WithConcurrency(cfg.WorkerConcurrency),
		jobs.WithPollInterval(cfg.WorkerPollEvery),
	)

	log.Info("worker started", "env", cfg.Env, "types", registry.Types())
	poller.Run(ctx)
	log.Info("worker shutting down")
}

func workerName(env string) string {
	host, _ := os.Hostname()
	return env + "/" + host + "/" + uuid.NewString()[:8]
}
