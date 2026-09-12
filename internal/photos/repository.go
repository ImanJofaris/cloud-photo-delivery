package photos

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("photo not found")

type CreateInput struct {
	EventID           uuid.UUID
	StorageKey        string
	OriginalFilename  string
	MimeType          string
	FileSize          int64
	Status            Status
	UploadKind        UploadKind
	MultipartUploadID *string
}

type Repository interface {
	Create(ctx context.Context, in CreateInput) (*Photo, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Photo, error)
	GetByIDForEvent(ctx context.Context, eventID, id uuid.UUID) (*Photo, error)
	// MarkProcessing transitions UPLOADING -> PROCESSING exactly once and
	// increments the owning event's counters in the same transaction.
	MarkProcessing(ctx context.Context, id uuid.UUID, incrementBytes int64) (*Photo, bool, error)
	// MarkReady records derivatives and dimensions and transitions
	// PROCESSING -> READY. It is idempotent: re-running overwrites the same
	// keys. Returns ErrNotFound if the photo no longer exists.
	MarkReady(ctx context.Context, id uuid.UUID, width, height int, d Derivatives) error
	MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
	SavePartETag(ctx context.Context, photoID uuid.UUID, partNumber int, etag string) error
	// EventOwner returns the owning user for an event, used by the worker to
	// rebuild deterministic storage keys.
	EventOwner(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const photoColumns = `id, event_id, storage_key, thumbnail_key, optimized_key, medium_key,
	original_filename, mime_type, file_size, width, height, status, upload_kind,
	multipart_upload_id, error_message, created_at, updated_at`

func scanPhoto(row pgx.Row) (*Photo, error) {
	var p Photo
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

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *PostgresRepository) Create(ctx context.Context, in CreateInput) (*Photo, error) {
	row := r.pool.QueryRow(ctx,
		`INSERT INTO photos (event_id, storage_key, original_filename, mime_type, file_size, status, upload_kind, multipart_upload_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+photoColumns,
		in.EventID, in.StorageKey, nullString(in.OriginalFilename), in.MimeType, in.FileSize,
		in.Status, in.UploadKind, in.MultipartUploadID)
	return scanPhoto(row)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Photo, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+photoColumns+` FROM photos WHERE id = $1`, id)
	return scanPhoto(row)
}

func (r *PostgresRepository) GetByIDForEvent(ctx context.Context, eventID, id uuid.UUID) (*Photo, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+photoColumns+` FROM photos WHERE id = $1 AND event_id = $2`, id, eventID)
	return scanPhoto(row)
}

func (r *PostgresRepository) MarkProcessing(ctx context.Context, id uuid.UUID, incrementBytes int64) (*Photo, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx,
		`UPDATE photos SET status = $2, updated_at = NOW()
		 WHERE id = $1 AND status = $3
		 RETURNING `+photoColumns,
		id, StatusProcessing, StatusUploading)
	photo, err := scanPhoto(row)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			current, getErr := r.getTx(ctx, tx, id)
			if getErr != nil {
				return nil, false, getErr
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, false, err
			}
			return current, false, nil
		}
		return nil, false, err
	}

	if incrementBytes > 0 {
		_, err = tx.Exec(ctx,
			`UPDATE events SET photo_count = photo_count + 1, storage_bytes = storage_bytes + $2, updated_at = NOW()
			 WHERE id = $1`,
			photo.EventID, incrementBytes)
		if err != nil {
			return nil, false, err
		}
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE events SET photo_count = photo_count + 1, updated_at = NOW() WHERE id = $1`,
			photo.EventID)
		if err != nil {
			return nil, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return photo, true, nil
}

func (r *PostgresRepository) getTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*Photo, error) {
	row := tx.QueryRow(ctx, `SELECT `+photoColumns+` FROM photos WHERE id = $1`, id)
	return scanPhoto(row)
}

func (r *PostgresRepository) MarkReady(ctx context.Context, id uuid.UUID, width, height int, d Derivatives) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE photos SET status = $2, width = $3, height = $4,
			thumbnail_key = $5, medium_key = $6, optimized_key = $7,
			error_message = NULL, updated_at = NOW()
		 WHERE id = $1`,
		id, StatusReady, width, height, d.Thumbnail.Key, d.Medium.Key, d.Optimized.Key)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE photos SET status = $2, error_message = $3, updated_at = NOW()
		 WHERE id = $1 AND status <> $4`, id, StatusFailed, msg, StatusReady)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PostgresRepository) SavePartETag(ctx context.Context, photoID uuid.UUID, partNumber int, etag string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO upload_parts (photo_id, part_number, etag) VALUES ($1, $2, $3)
		 ON CONFLICT (photo_id, part_number) DO UPDATE SET etag = EXCLUDED.etag`,
		photoID, partNumber, etag)
	return err
}

func (r *PostgresRepository) EventOwner(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error) {
	var userID uuid.UUID
	err := r.pool.QueryRow(ctx, `SELECT user_id FROM events WHERE id = $1`, eventID).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return userID, nil
}
