package events

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("event not found")
	ErrSlugTaken = errors.New("slug already taken")
)

type ListFilter struct {
	Status *Status
	Query  string
	Cursor *Cursor
	Limit  int
}

type CreateInput struct {
	UserID      uuid.UUID
	Name        string
	Slug        string
	ClientName  string
	ClientEmail string
	Location    string
	Description string
	EventDate   *time.Time
	Status      Status
	ExpiresAt   *time.Time
	Settings    Settings
}

type UpdateInput struct {
	Name        *string
	ClientName  *string
	ClientEmail *string
	Location    *string
	Description *string
	EventDate   *time.Time
	ClearDate   bool
	ExpiresAt   *time.Time
	ClearExpiry bool
}

type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Event, *Settings, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*Event, error)
	List(ctx context.Context, userID uuid.UUID, f ListFilter) ([]*Event, error)
	Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (*Event, error)
	SetStatus(ctx context.Context, userID, id uuid.UUID, status Status) (*Event, error)
	SoftDelete(ctx context.Context, userID, id uuid.UUID) error
	CountByStatus(ctx context.Context, userID uuid.UUID, status Status) (int, error)
	SlugExists(ctx context.Context, userID uuid.UUID, slug string) (bool, error)

	GetSettings(ctx context.Context, userID, eventID uuid.UUID) (*Settings, error)
	UpdateSettings(ctx context.Context, userID, eventID uuid.UUID, s Settings) (*Settings, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const eventColumns = `id, user_id, name, slug, COALESCE(client_name, ''), COALESCE(client_email, ''),
	COALESCE(location, ''), COALESCE(description, ''), event_date, status, cover_photo_id,
	storage_bytes, photo_count, guest_count, expires_at, deleted_at, created_at, updated_at`

func scanEvent(row pgx.Row) (*Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.UserID, &e.Name, &e.Slug, &e.ClientName, &e.ClientEmail,
		&e.Location, &e.Description, &e.EventDate, &e.Status, &e.CoverPhotoID,
		&e.StorageBytes, &e.PhotoCount, &e.GuestCount, &e.ExpiresAt, &e.DeletedAt,
		&e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Event, *Settings, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var eventID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO events (user_id, name, slug, client_name, client_email, location, description, event_date, status, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id`,
		in.UserID, in.Name, in.Slug, nullString(in.ClientName), nullString(in.ClientEmail),
		nullString(in.Location), nullString(in.Description), in.EventDate, in.Status, in.ExpiresAt,
	).Scan(&eventID)
	if err != nil {
		return nil, nil, err
	}

	s := in.Settings
	_, err = tx.Exec(ctx,
		`INSERT INTO event_settings (event_id, visibility, password_hash, allow_download, allow_original_download, watermark_enabled)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		eventID, s.Visibility, nullString(s.PasswordHash), s.AllowDownload, s.AllowOriginalDownload, s.WatermarkEnabled)
	if err != nil {
		return nil, nil, err
	}

	row := tx.QueryRow(ctx, `SELECT `+eventColumns+` FROM events WHERE id = $1`, eventID)
	created, err := scanEvent(row)
	if err != nil {
		return nil, nil, err
	}

	settings, err := getSettingsTx(ctx, tx, eventID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return created, settings, nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*Event, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM events
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID)
	return scanEvent(row)
}

func (r *PostgresRepository) List(ctx context.Context, userID uuid.UUID, f ListFilter) ([]*Event, error) {
	limit := NormalizeLimit(f.Limit)
	args := []any{userID}
	q := `SELECT ` + eventColumns + ` FROM events WHERE user_id = $1 AND deleted_at IS NULL`

	if f.Status != nil {
		args = append(args, string(*f.Status))
		q += ` AND status = $` + itoa(len(args))
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		q += ` AND name ILIKE $` + itoa(len(args))
	}
	if f.Cursor != nil {
		args = append(args, f.Cursor.CreatedAt, f.Cursor.ID)
		q += ` AND (created_at, id) < ($` + itoa(len(args)-1) + `, $` + itoa(len(args)) + `)`
	}

	args = append(args, limit)
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) Update(ctx context.Context, userID, id uuid.UUID, in UpdateInput) (*Event, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE events SET
			name = COALESCE($3, name),
			client_name = COALESCE($4, client_name),
			client_email = COALESCE($5, client_email),
			location = COALESCE($6, location),
			description = COALESCE($7, description),
			event_date = CASE WHEN $8 THEN NULL ELSE COALESCE($9, event_date) END,
			expires_at = CASE WHEN $10 THEN NULL ELSE COALESCE($11, expires_at) END,
			updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		 RETURNING `+eventColumns,
		id, userID, in.Name, in.ClientName, in.ClientEmail, in.Location, in.Description,
		in.ClearDate, in.EventDate, in.ClearExpiry, in.ExpiresAt)
	return scanEvent(row)
}

func (r *PostgresRepository) SetStatus(ctx context.Context, userID, id uuid.UUID, status Status) (*Event, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE events SET status = $3, updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
		 RETURNING `+eventColumns, id, userID, status)
	return scanEvent(row)
}

func (r *PostgresRepository) SoftDelete(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE events SET deleted_at = NOW(), updated_at = NOW()
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) CountByStatus(ctx context.Context, userID uuid.UUID, status Status) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM events
		 WHERE user_id = $1 AND status = $2 AND deleted_at IS NULL`, userID, status).Scan(&n)
	return n, err
}

func (r *PostgresRepository) SlugExists(ctx context.Context, userID uuid.UUID, slug string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM events WHERE user_id = $1 AND slug = $2)`, userID, slug).Scan(&exists)
	return exists, err
}

const settingsColumns = `event_id, visibility, COALESCE(password_hash, ''), allow_download,
	allow_original_download, watermark_enabled, updated_at`

func scanSettings(row pgx.Row) (*Settings, error) {
	var s Settings
	err := row.Scan(&s.EventID, &s.Visibility, &s.PasswordHash, &s.AllowDownload,
		&s.AllowOriginalDownload, &s.WatermarkEnabled, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func getSettingsTx(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (*Settings, error) {
	row := tx.QueryRow(ctx, `SELECT `+settingsColumns+` FROM event_settings WHERE event_id = $1`, eventID)
	return scanSettings(row)
}

func (r *PostgresRepository) GetSettings(ctx context.Context, userID, eventID uuid.UUID) (*Settings, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT s.event_id, s.visibility, COALESCE(s.password_hash, ''), s.allow_download,
			s.allow_original_download, s.watermark_enabled, s.updated_at
		 FROM event_settings s
		 JOIN events e ON e.id = s.event_id
		 WHERE s.event_id = $1 AND e.user_id = $2 AND e.deleted_at IS NULL`, eventID, userID)
	return scanSettings(row)
}

func (r *PostgresRepository) UpdateSettings(ctx context.Context, userID, eventID uuid.UUID, s Settings) (*Settings, error) {
	row := r.pool.QueryRow(ctx,
		`UPDATE event_settings SET
			visibility = $3,
			password_hash = $4,
			allow_download = $5,
			allow_original_download = $6,
			watermark_enabled = $7,
			updated_at = NOW()
		 WHERE event_id = $1
		   AND EXISTS (
			   SELECT 1 FROM events e
			   WHERE e.id = event_settings.event_id AND e.user_id = $2 AND e.deleted_at IS NULL
		   )
		 RETURNING `+settingsColumns, eventID, userID, s.Visibility, nullString(s.PasswordHash),
		s.AllowDownload, s.AllowOriginalDownload, s.WatermarkEnabled)
	return scanSettings(row)
}
