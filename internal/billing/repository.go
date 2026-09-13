package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound                 = errors.New("billing: not found")
	ErrActiveSubscriptionExists = errors.New("billing: non-terminal subscription already exists")
)

type CreateSubscriptionInput struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	PlanID             string
	Status             Status
	Provider           string
	ProviderRef        string
	Interval           string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
}

type CreateInvoiceInput struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	SubscriptionID *uuid.UUID
	AmountCents    int
	Currency       string
	Status         InvoiceStatus
	ProviderRef    string
}

type Repository interface {
	ListPlans(ctx context.Context) ([]*Plan, error)
	GetPlan(ctx context.Context, id string) (*Plan, error)

	GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error)
	GetLatestSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error)
	GetSubscriptionByProviderRef(ctx context.Context, ref string) (*Subscription, error)
	CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error)
	UpdateSubscriptionPlan(ctx context.Context, id uuid.UUID, planID, interval string) (*Subscription, error)
	SetCancelAtPeriodEnd(ctx context.Context, id uuid.UUID, cancel bool) (*Subscription, error)
	SetStatus(ctx context.Context, id uuid.UUID, status Status) (*Subscription, error)

	CreateInvoice(ctx context.Context, in CreateInvoiceInput) (*Invoice, error)
	GetInvoiceByProviderRef(ctx context.Context, ref string) (*Invoice, error)
	GetLatestOpenInvoice(ctx context.Context, subscriptionID uuid.UUID) (*Invoice, error)
	MarkInvoicePaid(ctx context.Context, id uuid.UUID, paidAt time.Time) error
	ListInvoices(ctx context.Context, userID uuid.UUID, cursor *InvoiceCursor, limit int) ([]*Invoice, error)

	// RecordWebhookEvent returns false when the provider event was already seen.
	RecordWebhookEvent(ctx context.Context, provider, providerRef, eventType string) (bool, error)
	DeleteWebhookEvent(ctx context.Context, provider, providerRef string) error

	CountActiveEvents(ctx context.Context, userID uuid.UUID) (int, error)
	UserStorageBytes(ctx context.Context, userID uuid.UUID) (int64, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const planColumns = `id, name, price_cents, currency, interval, limits, active`

func scanPlan(row pgx.Row) (*Plan, error) {
	var p Plan
	var raw []byte
	err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Currency, &p.Interval, &raw, &p.Active)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(raw, &p.Limits); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PostgresRepository) ListPlans(ctx context.Context) ([]*Plan, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+planColumns+` FROM plans WHERE active = TRUE ORDER BY price_cents ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Plan, 0, 4)
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) GetPlan(ctx context.Context, id string) (*Plan, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+planColumns+` FROM plans WHERE id = $1`, id)
	return scanPlan(row)
}

const subscriptionColumns = `id, user_id, plan_id, status, COALESCE(provider, ''), provider_ref,
	interval, current_period_start, current_period_end, cancel_at_period_end, created_at, updated_at`

func scanSubscription(row pgx.Row) (*Subscription, error) {
	var s Subscription
	err := row.Scan(&s.ID, &s.UserID, &s.PlanID, &s.Status, &s.Provider, &s.ProviderRef,
		&s.Interval, &s.CurrentPeriodStart, &s.CurrentPeriodEnd, &s.CancelAtPeriodEnd,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepository) GetActiveSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+subscriptionColumns+` FROM subscriptions
		 WHERE user_id = $1 AND status IN ('trialing', 'active', 'past_due')
		   AND (current_period_end IS NULL OR current_period_end > NOW())
		 ORDER BY created_at DESC LIMIT 1`, userID)
	return scanSubscription(row)
}

func (r *PostgresRepository) GetLatestSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+subscriptionColumns+` FROM subscriptions
		 WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, userID)
	return scanSubscription(row)
}

func (r *PostgresRepository) GetSubscriptionByProviderRef(ctx context.Context, ref string) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+subscriptionColumns+` FROM subscriptions WHERE provider_ref = $1`, ref)
	return scanSubscription(row)
}

func (r *PostgresRepository) CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO subscriptions (id, user_id, plan_id, status, provider, provider_ref, interval,
			current_period_start, current_period_end)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+subscriptionColumns,
		in.ID, in.UserID, in.PlanID, in.Status, in.Provider, nullString(in.ProviderRef), in.Interval,
		in.CurrentPeriodStart, in.CurrentPeriodEnd)

	s, err := scanSubscription(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_subs_non_terminal_user" {
			return nil, ErrActiveSubscriptionExists
		}
		return nil, err
	}
	return s, nil
}

func (r *PostgresRepository) UpdateSubscriptionPlan(ctx context.Context, id uuid.UUID, planID, interval string) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE subscriptions SET plan_id = $2, interval = $3, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+subscriptionColumns, id, planID, interval)
	return scanSubscription(row)
}

func (r *PostgresRepository) SetCancelAtPeriodEnd(ctx context.Context, id uuid.UUID, cancel bool) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE subscriptions SET cancel_at_period_end = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+subscriptionColumns, id, cancel)
	return scanSubscription(row)
}

func (r *PostgresRepository) SetStatus(ctx context.Context, id uuid.UUID, status Status) (*Subscription, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE subscriptions SET status = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+subscriptionColumns, id, status)
	return scanSubscription(row)
}

const invoiceColumns = `id, user_id, subscription_id, amount_cents, currency, status, provider_ref, issued_at, paid_at`

func scanInvoice(row pgx.Row) (*Invoice, error) {
	var inv Invoice
	err := row.Scan(&inv.ID, &inv.UserID, &inv.SubscriptionID, &inv.AmountCents, &inv.Currency,
		&inv.Status, &inv.ProviderRef, &inv.IssuedAt, &inv.PaidAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &inv, nil
}

func (r *PostgresRepository) CreateInvoice(ctx context.Context, in CreateInvoiceInput) (*Invoice, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO invoices (id, user_id, subscription_id, amount_cents, currency, status, provider_ref)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+invoiceColumns,
		in.ID, in.UserID, in.SubscriptionID, in.AmountCents, in.Currency, in.Status, nullString(in.ProviderRef))
	return scanInvoice(row)
}

func (r *PostgresRepository) GetInvoiceByProviderRef(ctx context.Context, ref string) (*Invoice, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+invoiceColumns+` FROM invoices WHERE provider_ref = $1`, ref)
	return scanInvoice(row)
}

func (r *PostgresRepository) GetLatestOpenInvoice(ctx context.Context, subscriptionID uuid.UUID) (*Invoice, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+invoiceColumns+` FROM invoices
		 WHERE subscription_id = $1 AND status = 'open'
		 ORDER BY issued_at DESC LIMIT 1`, subscriptionID)
	return scanInvoice(row)
}

func (r *PostgresRepository) MarkInvoicePaid(ctx context.Context, id uuid.UUID, paidAt time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE invoices SET status = 'paid', paid_at = $2 WHERE id = $1`, id, paidAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) ListInvoices(ctx context.Context, userID uuid.UUID, cursor *InvoiceCursor, limit int) ([]*Invoice, error) {
	args := []any{userID}
	q := `SELECT ` + invoiceColumns + ` FROM invoices WHERE user_id = $1`
	if cursor != nil {
		args = append(args, cursor.IssuedAt, cursor.ID)
		q += ` AND (issued_at, id) < ($` + strconv.Itoa(len(args)-1) + `, $` + strconv.Itoa(len(args)) + `)`
	}
	args = append(args, limit)
	q += ` ORDER BY issued_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Invoice, 0, limit)
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) RecordWebhookEvent(ctx context.Context, provider, providerRef, eventType string) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO billing_webhook_events (provider, provider_ref, event_type)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (provider, provider_ref) DO NOTHING`, provider, providerRef, eventType)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) DeleteWebhookEvent(ctx context.Context, provider, providerRef string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM billing_webhook_events WHERE provider = $1 AND provider_ref = $2`, provider, providerRef)
	return err
}

func (r *PostgresRepository) CountActiveEvents(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM events WHERE user_id = $1 AND status = 'active' AND deleted_at IS NULL`,
		userID).Scan(&count)
	return count, err
}

func (r *PostgresRepository) UserStorageBytes(ctx context.Context, userID uuid.UUID) (int64, error) {
	var bytes int64
	err := r.pool.QueryRow(ctx, `SELECT storage_bytes FROM users WHERE id = $1`, userID).Scan(&bytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return bytes, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
