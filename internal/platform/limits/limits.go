package limits

import "context"

// PlanLimits is the boundary through which domains ask whether a tenant may
// use more of a metered resource. Billing (Phase 8) supplies the real
// implementation; Default allows everything. A limit of zero means unlimited.
type PlanLimits interface {
	MaxActiveEvents(ctx context.Context, userID string) (int, error)
	MaxPhotosPerEvent(ctx context.Context, userID string) (int, error)
	MaxStorageBytes(ctx context.Context, userID string) (int64, error)
}

// Default grants unlimited usage.
type Default struct{}

func NewDefault() Default { return Default{} }

func (Default) MaxActiveEvents(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (Default) MaxPhotosPerEvent(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (Default) MaxStorageBytes(ctx context.Context, userID string) (int64, error) {
	return 0, nil
}
