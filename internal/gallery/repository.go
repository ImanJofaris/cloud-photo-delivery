package gallery

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/events"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("gallery resource not found")

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const eventColumns = `id, user_id, name, slug, COALESCE(client_name, ''), COALESCE(client_email, ''),
	COALESCE(location, ''), COALESCE(description, ''), event_date, status, cover_photo_id,
	storage_bytes, photo_count, guest_count, expires_at, deleted_at, created_at, updated_at`

const settingsColumns = `event_id, visibility, COALESCE(password_hash, ''), allow_download,
	allow_original_download, watermark_enabled, password_changed_at, updated_at`

func scanEvent(row pgx.Row) (*events.Event, error) {
	var e events.Event
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

func scanSettings(row pgx.Row) (*events.Settings, error) {
	var s events.Settings
	err := row.Scan(&s.EventID, &s.Visibility, &s.PasswordHash, &s.AllowDownload,
		&s.AllowOriginalDownload, &s.WatermarkEnabled, &s.PasswordChangedAt, &s.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

// EventBySlug looks up a visible (not soft-deleted, not expired) event by slug.
func (r *PostgresRepository) EventBySlug(ctx context.Context, slug string) (*events.Event, *events.Settings, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+eventColumns+` FROM events
		 WHERE slug = $1 AND deleted_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`, slug)
	event, err := scanEvent(row)
	if err != nil {
		return nil, nil, err
	}

	srow := r.pool.QueryRow(ctx, `SELECT `+settingsColumns+` FROM event_settings WHERE event_id = $1`, event.ID)
	settings, err := scanSettings(srow)
	if err != nil {
		return nil, nil, err
	}
	return event, settings, nil
}

// GetSettingsByEventID returns settings for a specific event.
func (r *PostgresRepository) GetSettingsByEventID(ctx context.Context, eventID uuid.UUID) (*events.Settings, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+settingsColumns+` FROM event_settings WHERE event_id = $1`, eventID)
	return scanSettings(row)
}

const photoColumns = `id, event_id, storage_key, thumbnail_key, optimized_key, medium_key,
	original_filename, mime_type, file_size, width, height, status, upload_kind,
	multipart_upload_id, error_message, created_at, updated_at`

func scanPhoto(row pgx.Row) (*photos.Photo, error) {
	var p photos.Photo
	err := row.Scan(&p.ID, &p.EventID, &p.StorageKey, &p.ThumbnailKey, &p.OptimizedKey, &p.MediumKey,
		&p.OriginalFilename, &p.MimeType, &p.FileSize, &p.Width, &p.Height, &p.Status, &p.UploadKind,
		&p.MultipartUploadID, &p.ErrorMessage, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

// ListReadyPhotos returns READY photos for an event using keyset pagination.
func (r *PostgresRepository) ListReadyPhotos(ctx context.Context, eventID uuid.UUID, cursor *Cursor, limit int) ([]*photos.Photo, error) {
	args := []any{eventID, photos.StatusReady}
	q := `SELECT ` + photoColumns + ` FROM photos WHERE event_id = $1 AND status = $2`
	if cursor != nil {
		args = append(args, cursor.CreatedAt, cursor.ID)
		q += ` AND (created_at, id) < ($3, $4)`
	}
	args = append(args, limit)
	q += ` ORDER BY created_at DESC, id DESC LIMIT $` + itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*photos.Photo
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReadyPhoto returns a single READY photo scoped to the event.
func (r *PostgresRepository) ReadyPhoto(ctx context.Context, eventID, photoID uuid.UUID) (*photos.Photo, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+photoColumns+` FROM photos
		 WHERE id = $1 AND event_id = $2 AND status = $3`, photoID, eventID, photos.StatusReady)
	return scanPhoto(row)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
