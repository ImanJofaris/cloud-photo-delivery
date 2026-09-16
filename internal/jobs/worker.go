package jobs

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

type Poller struct {
	queue       Queue
	registry    *Registry
	log         *slog.Logger
	workerID    string
	concurrency int
	pollEvery   time.Duration
	observer    Observer
}

type Option func(*Poller)

func WithConcurrency(n int) Option {
	return func(p *Poller) {
		if n > 0 {
			p.concurrency = n
		}
	}
}

func WithPollInterval(d time.Duration) Option {
	return func(p *Poller) {
		if d > 0 {
			p.pollEvery = d
		}
	}
}

// WithObserver attaches a metrics observer to job outcomes. Nil is a no-op.
func WithObserver(obs Observer) Option {
	return func(p *Poller) {
		p.observer = obs
	}
}

func NewPoller(queue Queue, registry *Registry, log *slog.Logger, workerID string, opts ...Option) *Poller {
	p := &Poller{
		queue:       queue,
		registry:    registry,
		log:         log,
		workerID:    workerID,
		concurrency: 1,
		pollEvery:   time.Second,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Run polls until ctx is cancelled, then waits for in-flight jobs to release
// and finish. A graceful shutdown leaves no job stuck in running: it either
// completes, or is released back to pending.
func (p *Poller) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.loop(ctx)
		}()
	}
	p.log.Info("worker polling", "worker", p.workerID, "concurrency", p.concurrency,
		"types", p.registry.Types())
	wg.Wait()
	p.log.Info("worker stopped", "worker", p.workerID)
}

func (p *Poller) loop(ctx context.Context) {
	ticker := time.NewTicker(p.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		job, err := p.queue.Claim(ctx, p.workerID)
		if err != nil {
			if errors.Is(err, ErrNoJob) {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				continue
			}
			if ctx.Err() != nil {
				return
			}
			p.log.Error("claim failed", "worker", p.workerID, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			continue
		}

		p.handle(ctx, job)
	}
}

func (p *Poller) handle(ctx context.Context, job *Job) {
	start := time.Now()
	log := p.log.With("job_id", job.ID, "job_type", job.Type, "attempt", job.Attempts+1)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := p.registry.Dispatch(runCtx, job)
	duration := time.Since(start)

	if err == nil {
		if cerr := p.queue.Complete(ctx, job.ID); cerr != nil {
			log.Error("complete failed", "error", cerr)
			return
		}
		p.observe(job.Type, OutcomeDone, duration)
		log.Info("job done", "duration_ms", duration.Milliseconds())
		return
	}

	if errors.Is(err, context.Canceled) && ctx.Err() != nil {
		if rerr := p.queue.Release(context.WithoutCancel(ctx), job.ID); rerr != nil {
			log.Error("release failed", "error", rerr)
		}
		p.observe(job.Type, OutcomeReleased, duration)
		log.Info("job released on shutdown")
		return
	}

	retry, ferr := p.queue.Fail(context.WithoutCancel(ctx), job.ID, err.Error())
	if ferr != nil {
		log.Error("fail transition failed", "error", ferr)
		return
	}
	if retry {
		p.observe(job.Type, OutcomeRetry, duration)
		log.Warn("job failed, will retry", "error", err.Error(),
			"duration_ms", duration.Milliseconds())
		return
	}
	p.observe(job.Type, OutcomeFailed, duration)
	log.Error("job failed permanently", "error", err.Error(),
		"duration_ms", duration.Milliseconds())
}

func (p *Poller) observe(jobType, outcome string, d time.Duration) {
	if p.observer != nil {
		p.observer.RecordJob(jobType, outcome, d)
	}
}
