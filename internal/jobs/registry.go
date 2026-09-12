package jobs

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

type Handler func(ctx context.Context, payload []byte) error

// Registry maps a job type to its handler. It is only mutated during startup
// and read by workers, so no locking is required.
type Registry struct {
	handlers map[string]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

func (r *Registry) Register(jobType string, h Handler) *Registry {
	r.handlers[jobType] = h
	return r
}

func (r *Registry) Lookup(jobType string) (Handler, bool) {
	h, ok := r.handlers[jobType]
	return h, ok
}

func (r *Registry) Types() []string {
	out := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// ErrUnknownJobType is returned when a claimed job has no registered handler.
var ErrUnknownJobType = errors.New("unknown job type")

func (r *Registry) Dispatch(ctx context.Context, job *Job) error {
	h, ok := r.Lookup(job.Type)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownJobType, job.Type)
	}
	return h(ctx, job.Payload)
}
