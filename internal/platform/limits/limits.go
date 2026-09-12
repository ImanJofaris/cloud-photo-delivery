package limits

import "context"

// PlanLimits is the boundary through which the events domain asks whether a
// tenant may create or activate more events. Billing (Phase 8) supplies the
// real implementation; until then Default allows everything.
type PlanLimits interface {
	MaxActiveEvents(ctx context.Context, userID string) (int, error)
}

// Default grants an unlimited number of active events.
type Default struct{}

func NewDefault() Default { return Default{} }

func (Default) MaxActiveEvents(ctx context.Context, userID string) (int, error) {
	return 0, nil
}
