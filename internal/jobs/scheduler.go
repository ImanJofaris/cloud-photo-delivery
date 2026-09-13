package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

type UniqueEnqueuer interface {
	EnqueueUnique(ctx context.Context, jobType string, payload any) (uuid.UUID, bool, error)
}

type Schedule struct {
	Type  string
	Every time.Duration
}

// Scheduler enqueues recurring jobs on a ticker so no external cron is needed.
// Every schedule fires once at startup and then on its interval; EnqueueUnique
// keeps at most one pending/running instance per job type.
type Scheduler struct {
	queue     UniqueEnqueuer
	log       *slog.Logger
	schedules []Schedule
}

func NewScheduler(queue UniqueEnqueuer, log *slog.Logger, schedules ...Schedule) *Scheduler {
	return &Scheduler{queue: queue, log: log, schedules: schedules}
}

func (s *Scheduler) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, sched := range s.schedules {
		wg.Add(1)
		go func(sch Schedule) {
			defer wg.Done()
			if sch.Every <= 0 {
				return
			}
			s.enqueue(ctx, sch.Type)
			ticker := time.NewTicker(sch.Every)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.enqueue(ctx, sch.Type)
				}
			}
		}(sched)
	}
	wg.Wait()
}

// Tick runs every schedule once. Exposed for tests and callers that manage
// their own timing.
func (s *Scheduler) Tick(ctx context.Context) {
	for _, sched := range s.schedules {
		s.enqueue(ctx, sched.Type)
	}
}

func (s *Scheduler) enqueue(ctx context.Context, jobType string) {
	_, enqueued, err := s.queue.EnqueueUnique(ctx, jobType, map[string]any{})
	if err != nil {
		s.log.Error("scheduled job enqueue failed", "job_type", jobType, "error", err)
		return
	}
	if enqueued {
		s.log.Info("scheduled job enqueued", "job_type", jobType)
	}
}
