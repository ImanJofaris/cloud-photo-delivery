package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRefreshNotFound = errors.New("refresh token not found")
	ErrResetNotFound   = errors.New("reset token not found")
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type Repository interface {
	CreateRefreshToken(ctx context.Context, userID, familyID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id uuid.UUID) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error

	CreatePasswordReset(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error
	GetPasswordResetByHash(ctx context.Context, tokenHash string) (*PasswordReset, error)
	MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error
}

type PasswordReset struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	UsedAt    *time.Time
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateRefreshToken(ctx context.Context, userID, familyID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (user_id, family_id, token_hash, expires_at)
		 VALUES ($1, $2, $3, $4)`, userID, familyID, tokenHash, expiresAt)
	return err
}

func (r *PostgresRepository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	var rt RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, family_id, expires_at, revoked_at
		 FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&rt.ID, &rt.UserID, &rt.FamilyID, &rt.ExpiresAt, &rt.RevokedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRefreshNotFound
		}
		return nil, err
	}
	return &rt, nil
}

func (r *PostgresRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}

func (r *PostgresRepository) RevokeFamily(ctx context.Context, familyID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE family_id = $1 AND revoked_at IS NULL`, familyID)
	return err
}

func (r *PostgresRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (r *PostgresRepository) CreatePasswordReset(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO password_resets (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt)
	return err
}

func (r *PostgresRepository) GetPasswordResetByHash(ctx context.Context, tokenHash string) (*PasswordReset, error) {
	var pr PasswordReset
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, expires_at, used_at
		 FROM password_resets WHERE token_hash = $1`, tokenHash).
		Scan(&pr.ID, &pr.UserID, &pr.ExpiresAt, &pr.UsedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrResetNotFound
		}
		return nil, err
	}
	return &pr, nil
}

func (r *PostgresRepository) MarkPasswordResetUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE password_resets SET used_at = NOW() WHERE id = $1 AND used_at IS NULL`, id)
	return err
}
