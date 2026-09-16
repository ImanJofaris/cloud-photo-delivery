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
	"github.com/stretchr/testify/require"
)

type fakeJob struct {
	id      uuid.UUID
	jobType string
	payload []byte
}

type fakeQueue struct {
	mu       sync.Mutex
	jobs     []fakeJob
	done     []uuid.UUID
	failed   []string
	released []uuid.UUID
	claimErr error
}

func (f *fakeQueue) Claim(_ context.Context, _ string) (*Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	if len(f.jobs) == 0 {
		return nil, ErrNoJob
	}
	j := f.jobs[0]
	f.jobs = f.jobs[1:]
	return &Job{ID: j.id, Type: j.jobType, Payload: j.payload}, nil
}

func (f *fakeQueue) Complete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.done = append(f.done, id)
	return nil
}

func (f *fakeQueue) Fail(_ context.Context, id uuid.UUID, cause string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, cause)
	return true, nil
}

func (f *fakeQueue) Release(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, id)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestPoller_ProcessesJobToDone(t *testing.T) {
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "ok", payload: []byte("x")}}}
	reg := NewRegistry().Register("ok", func(context.Context, []byte) error { return nil })
	p := NewPoller(q, reg, discardLogger(), "w1", WithPollInterval(5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	waitFor(t, func() bool { q.mu.Lock(); defer q.mu.Unlock(); return len(q.done) == 1 })
	cancel()
	require.Empty(t, q.failed)
}

func TestPoller_FailureRecordsError(t *testing.T) {
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "bad"}}}
	reg := NewRegistry().Register("bad", func(context.Context, []byte) error { return errors.New("kaboom") })
	p := NewPoller(q, reg, discardLogger(), "w1", WithPollInterval(5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	waitFor(t, func() bool { q.mu.Lock(); defer q.mu.Unlock(); return len(q.failed) == 1 })
	cancel()
	require.Equal(t, "kaboom", q.failed[0])
}

func TestPoller_UnknownTypeIsFailure(t *testing.T) {
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "mystery"}}}
	p := NewPoller(q, NewRegistry(), discardLogger(), "w1", WithPollInterval(5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	waitFor(t, func() bool { q.mu.Lock(); defer q.mu.Unlock(); return len(q.failed) == 1 })
	cancel()
}

func TestPoller_ShutdownReleasesInFlightJob(t *testing.T) {
	started := make(chan struct{})
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "slow"}}}
	reg := NewRegistry().Register("slow", func(ctx context.Context, _ []byte) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	p := NewPoller(q, reg, discardLogger(), "w1", WithPollInterval(5*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	<-started
	cancel()
	<-done

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Len(t, q.released, 1, "in-flight job must be released on shutdown")
	require.Empty(t, q.failed, "shutdown must not count an attempt")
	require.Empty(t, q.done)
}

func TestPoller_ContextCancelledBeforeClaim(t *testing.T) {
	q := &fakeQueue{}
	p := NewPoller(q, NewRegistry(), discardLogger(), "w1", WithPollInterval(5*time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fin := make(chan struct{})
	go func() { p.Run(ctx); close(fin) }()
	select {
	case <-fin:
	case <-time.After(2 * time.Second):
		t.Fatal("poller did not stop after cancel")
	}
}

type fakeObserver struct {
	mu       sync.Mutex
	outcomes []string
}

func (f *fakeObserver) RecordJob(_ string, outcome string, _ time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outcomes = append(f.outcomes, outcome)
}

func TestPoller_RecordsDoneOutcome(t *testing.T) {
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "ok"}}}
	reg := NewRegistry().Register("ok", func(context.Context, []byte) error { return nil })
	obs := &fakeObserver{}
	p := NewPoller(q, reg, discardLogger(), "w1", WithPollInterval(5*time.Millisecond), WithObserver(obs))

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	waitFor(t, func() bool { q.mu.Lock(); defer q.mu.Unlock(); return len(q.done) == 1 })
	cancel()

	obs.mu.Lock()
	defer obs.mu.Unlock()
	require.Equal(t, []string{OutcomeDone}, obs.outcomes)
}

func TestPoller_RecordsRetryOutcome(t *testing.T) {
	q := &fakeQueue{jobs: []fakeJob{{id: uuid.New(), jobType: "bad"}}}
	reg := NewRegistry().Register("bad", func(context.Context, []byte) error { return errors.New("kaboom") })
	obs := &fakeObserver{}
	p := NewPoller(q, reg, discardLogger(), "w1", WithPollInterval(5*time.Millisecond), WithObserver(obs))

	ctx, cancel := context.WithCancel(context.Background())
	go p.Run(ctx)
	waitFor(t, func() bool { q.mu.Lock(); defer q.mu.Unlock(); return len(q.failed) == 1 })
	cancel()

	obs.mu.Lock()
	defer obs.mu.Unlock()
	require.Equal(t, []string{OutcomeRetry}, obs.outcomes)
}
