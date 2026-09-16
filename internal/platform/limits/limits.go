package limits

import "context"

// PlanLimits is the boundary through which domains ask whether a tenant may
// use more of a metered resource. Billing (Phase 8) supplies the real
// implementation; Default allows everything. A limit of zero means unlimited.
type PlanLimits interface {
	MaxEvents(ctx context.Context, userID string) (int, error)
	MaxPhotosPerEvent(ctx context.Context, userID string) (int, error)
	MaxStorageBytes(ctx context.Context, userID string) (int64, error)
	RetentionDays(ctx context.Context, userID string) (int, error)
	APIAccess(ctx context.Context, userID string) (bool, error)
	BrandingEnabled(ctx context.Context, userID string) (bool, error)
	OriginalDownloads(ctx context.Context, userID string) (bool, error)
}

// Default grants unlimited usage and every feature.
type Default struct{}

func NewDefault() Default { return Default{} }

func (Default) MaxEvents(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (Default) MaxPhotosPerEvent(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (Default) MaxStorageBytes(ctx context.Context, userID string) (int64, error) {
	return 0, nil
}

func (Default) RetentionDays(ctx context.Context, userID string) (int, error) {
	return 0, nil
}

func (Default) APIAccess(ctx context.Context, userID string) (bool, error) {
	return true, nil
}

func (Default) BrandingEnabled(ctx context.Context, userID string) (bool, error) {
	return true, nil
}

func (Default) OriginalDownloads(ctx context.Context, userID string) (bool, error) {
	return true, nil
}
