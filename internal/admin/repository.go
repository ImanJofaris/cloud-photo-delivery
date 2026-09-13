package admin

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) Stats(ctx context.Context) (*Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx,
		`SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM events WHERE deleted_at IS NULL),
			(SELECT COUNT(*) FROM photos WHERE status <> 'FAILED'),
			(SELECT COALESCE(SUM(storage_bytes), 0) FROM users),
			(SELECT COALESCE(SUM(amount_cents), 0) FROM invoices WHERE status = 'paid'),
			(SELECT COUNT(*) FROM subscriptions
			   WHERE status IN ('trialing', 'active', 'past_due')
			     AND (current_period_end IS NULL OR current_period_end > NOW()))`,
	).Scan(&s.Users, &s.Events, &s.Photos, &s.StorageBytes, &s.RevenueCents, &s.Subscriptions)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepository) ListUsers(ctx context.Context, cursor *Cursor, limit int) ([]*UserSummary, error) {
	args := []any{}
	q := `SELECT u.id, u.email, COALESCE(u.business_name, ''), u.is_admin, u.storage_bytes,
			(SELECT COUNT(*) FROM events e WHERE e.user_id = u.id AND e.deleted_at IS NULL),
			u.created_at
		 FROM users u`
	if cursor != nil {
		args = append(args, cursor.CreatedAt, cursor.ID)
		q += ` WHERE (u.created_at, u.id) < ($1, $2)`
	}
	args = append(args, limit)
	q += ` ORDER BY u.created_at DESC, u.id DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*UserSummary, 0, limit)
	for rows.Next() {
		var u UserSummary
		if err := rows.Scan(&u.ID, &u.Email, &u.BusinessName, &u.IsAdmin, &u.StorageBytes,
			&u.EventCount, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &u)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) ListSubscriptions(ctx context.Context, cursor *Cursor, limit int) ([]*SubscriptionSummary, error) {
	args := []any{}
	q := `SELECT s.id, s.user_id, u.email, s.plan_id, s.status, s.interval,
			s.current_period_end, s.cancel_at_period_end, s.created_at
		 FROM subscriptions s
		 JOIN users u ON u.id = s.user_id`
	if cursor != nil {
		args = append(args, cursor.CreatedAt, cursor.ID)
		q += ` WHERE (s.created_at, s.id) < ($1, $2)`
	}
	args = append(args, limit)
	q += ` ORDER BY s.created_at DESC, s.id DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*SubscriptionSummary, 0, limit)
	for rows.Next() {
		var s SubscriptionSummary
		if err := rows.Scan(&s.ID, &s.UserID, &s.UserEmail, &s.PlanID, &s.Status, &s.Interval,
			&s.CurrentPeriodEnd, &s.CancelAtPeriodEnd, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) QueueHealth(ctx context.Context) (*QueueHealth, error) {
	var q QueueHealth
	err := r.pool.QueryRow(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE status = 'pending'),
			COUNT(*) FILTER (WHERE status = 'running'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			MIN(run_at) FILTER (WHERE status = 'pending')
		 FROM jobs`,
	).Scan(&q.Pending, &q.Running, &q.Failed, &q.OldestPendingAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

func (r *PostgresRepository) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	var isAdmin bool
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE((SELECT is_admin FROM users WHERE id = $1), FALSE)`, userID).Scan(&isAdmin)
	return isAdmin, err
}

func (r *PostgresRepository) StorageTotals(ctx context.Context) (*StorageTotals, error) {
	var t StorageTotals
	err := r.pool.QueryRow(ctx,
		`SELECT
			(SELECT COALESCE(SUM(storage_bytes), 0) FROM users),
			(SELECT COUNT(*) FROM photos WHERE status <> 'UPLOADING')`,
	).Scan(&t.StorageBytes, &t.Photos)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
