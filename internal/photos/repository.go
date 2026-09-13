package photos

import (
	"context"
	"errors"
	"strconv"

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

// ListInput filters an owner-scoped photo listing.
type ListInput struct {
	EventID uuid.UUID
	UserID  uuid.UUID
	Cursor  *Cursor
	Status  Status
	Limit   int
}

// DeletedPhoto is the result of an owner-scoped delete: the storage keys that
// must be cleaned up and whether the event counters were decremented.
type DeletedPhoto struct {
	EventID uuid.UUID
	Keys    []string
	Counted bool
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
	// EventOwnedBy reports whether the user owns a non-deleted event.
	EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error)
	// ListByEvent returns the owner's photos, newest first.
	ListByEvent(ctx context.Context, in ListInput) ([]*Photo, error)
	// DeleteOwned removes a photo owned by the user and adjusts event
	// counters exactly once for photos that were counted at completion.
	DeleteOwned(ctx context.Context, photoID, userID uuid.UUID) (*DeletedPhoto, error)
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

func (r *PostgresRepository) EventOwnedBy(ctx context.Context, userID, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM events WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL)`,
		eventID, userID).Scan(&exists)
	return exists, err
}

const ownerPhotoColumns = `p.id, p.event_id, p.storage_key, p.thumbnail_key, p.optimized_key, p.medium_key,
	p.original_filename, p.mime_type, p.file_size, p.width, p.height, p.status, p.upload_kind,
	p.multipart_upload_id, p.error_message, p.created_at, p.updated_at`

func (r *PostgresRepository) ListByEvent(ctx context.Context, in ListInput) ([]*Photo, error) {
	args := []any{in.EventID, in.UserID}
	q := `SELECT ` + ownerPhotoColumns + ` FROM photos p
	      JOIN events e ON e.id = p.event_id
	      WHERE p.event_id = $1 AND e.user_id = $2 AND e.deleted_at IS NULL`
	if in.Status != "" {
		args = append(args, in.Status)
		q += ` AND p.status = $` + strconv.Itoa(len(args))
	}
	if in.Cursor != nil {
		args = append(args, in.Cursor.CreatedAt, in.Cursor.ID)
		q += ` AND (p.created_at, p.id) < ($` + strconv.Itoa(len(args)-1) + `, $` + strconv.Itoa(len(args)) + `)`
	}
	args = append(args, in.Limit)
	q += ` ORDER BY p.created_at DESC, p.id DESC LIMIT $` + strconv.Itoa(len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*Photo, 0, in.Limit)
	for rows.Next() {
		p, err := scanPhoto(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) DeleteOwned(ctx context.Context, photoID, userID uuid.UUID) (*DeletedPhoto, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out DeletedPhoto
	var status Status
	var fileSize int64
	var storageKey string
	var thumbnailKey, mediumKey, optimizedKey *string
	err = tx.QueryRow(ctx,
		`SELECT p.event_id, p.status, p.file_size, p.storage_key,
		        p.thumbnail_key, p.medium_key, p.optimized_key
		 FROM photos p
		 JOIN events e ON e.id = p.event_id
		 WHERE p.id = $1 AND e.user_id = $2 AND e.deleted_at IS NULL
		 FOR UPDATE OF p`,
		photoID, userID).Scan(&out.EventID, &status, &fileSize, &storageKey,
		&thumbnailKey, &mediumKey, &optimizedKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	out.Keys = appendKey(out.Keys, storageKey)
	out.Keys = appendKey(out.Keys, derefKey(thumbnailKey))
	out.Keys = appendKey(out.Keys, derefKey(mediumKey))
	out.Keys = appendKey(out.Keys, derefKey(optimizedKey))

	tag, err := tx.Exec(ctx, `DELETE FROM photos WHERE id = $1`, photoID)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}

	// Counters increment when the photo leaves UPLOADING (MarkProcessing), so
	// only photos that reached PROCESSING, READY, or FAILED decrement them.
	if status != StatusUploading {
		_, err = tx.Exec(ctx,
			`UPDATE events SET photo_count = GREATEST(photo_count - 1, 0),
			        storage_bytes = GREATEST(storage_bytes - $2, 0), updated_at = NOW()
			 WHERE id = $1`, out.EventID, fileSize)
		if err != nil {
			return nil, err
		}
		out.Counted = true
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &out, nil
}

func appendKey(keys []string, key string) []string {
	if key == "" {
		return keys
	}
	return append(keys, key)
}

func derefKey(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
