package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakeEnqueuer struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *fakeEnqueuer) EnqueueUnique(ctx context.Context, jobType string, payload any) (uuid.UUID, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return uuid.Nil, false, f.err
	}
	f.calls = append(f.calls, jobType)
	return uuid.New(), true, nil
}

func (f *fakeEnqueuer) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

func TestScheduler_TickEnqueuesEachSchedule(t *testing.T) {
	q := &fakeEnqueuer{}
	s := NewScheduler(q, testLogger(),
		Schedule{Type: "event.expire", Every: time.Minute},
		Schedule{Type: "event.purge", Every: time.Minute},
	)
	s.Tick(context.Background())

	got := q.called()
	if len(got) != 2 || got[0] != "event.expire" || got[1] != "event.purge" {
		t.Fatalf("calls = %v", got)
	}
}

func TestScheduler_TickSurvivesEnqueueError(t *testing.T) {
	q := &fakeEnqueuer{err: errors.New("db down")}
	s := NewScheduler(q, testLogger(), Schedule{Type: "event.expire", Every: time.Minute})
	s.Tick(context.Background())
	if len(q.called()) != 0 {
		t.Fatal("expected no recorded calls on error")
	}
}

func TestScheduler_RunStopsOnCancel(t *testing.T) {
	q := &fakeEnqueuer{}
	s := NewScheduler(q, testLogger(), Schedule{Type: "event.expire", Every: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for len(q.called()) < 2 {
		select {
		case <-deadline:
			t.Fatalf("expected repeated enqueues, got %v", q.called())
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("scheduler did not stop")
	}
}
