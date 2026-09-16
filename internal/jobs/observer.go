package jobs

import "time"

// Job outcomes reported to an Observer.
const (
	OutcomeDone     = "done"
	OutcomeRetry    = "retry"
	OutcomeFailed   = "failed"
	OutcomeReleased = "released"
)

// Observer receives job outcome metrics. It is called from every worker
// goroutine, so implementations must be safe for concurrent use.
type Observer interface {
	RecordJob(jobType, outcome string, d time.Duration)
}
