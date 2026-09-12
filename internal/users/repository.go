package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("user not found")
var ErrEmailTaken = errors.New("email already registered")

type Repository interface {
	Create(ctx context.Context, email, passwordHash, businessName string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	UpdateBusinessName(ctx context.Context, id uuid.UUID, name string) (*User, error)
	RecordFailedLogin(ctx context.Context, id uuid.UUID, maxFailed int, lockUntil time.Time) error
	ResetFailedLogin(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const userColumns = `id, email, password_hash, COALESCE(business_name, ''), email_verified_at,
	failed_login_count, locked_until, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.BusinessName, &u.EmailVerifiedAt,
		&u.FailedLoginCount, &u.LockedUntil, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

func (r *PostgresRepository) Create(ctx context.Context, email, passwordHash, businessName string) (*User, error) {
	var bn any
	if businessName != "" {
		bn = businessName
	}
	row := r.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, business_name)
		 VALUES ($1, $2, $3)
		 RETURNING `+userColumns, email, passwordHash, bn)

	u, err := scanUser(row)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return u, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
	return scanUser(row)
}

func (r *PostgresRepository) UpdateBusinessName(ctx context.Context, id uuid.UUID, name string) (*User, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE users SET business_name = $2, updated_at = NOW()
		 WHERE id = $1
		 RETURNING `+userColumns, id, name)
	return scanUser(row)
}

func (r *PostgresRepository) RecordFailedLogin(ctx context.Context, id uuid.UUID, maxFailed int, lockUntil time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET
			failed_login_count = failed_login_count + 1,
			locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN $3 ELSE locked_until END,
			updated_at = NOW()
		 WHERE id = $1`, id, maxFailed, lockUntil)
	return err
}

func (r *PostgresRepository) ResetFailedLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET failed_login_count = 0, locked_until = NULL, updated_at = NOW()
		 WHERE id = $1`, id)
	return err
}

func (r *PostgresRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = NOW() WHERE id = $1`, id, passwordHash)
	return err
}
