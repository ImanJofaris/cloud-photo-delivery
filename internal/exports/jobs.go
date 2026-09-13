package exports

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/photos"
	"github.com/imanjofaris/cloud-photo-delivery/pkg/r2"
)

const (
	photoPageSize          = 500
	cleanupBatchSize       = 100
	staleProcessingTimeout = time.Hour
)

type EventOwner func(ctx context.Context, eventID uuid.UUID) (uuid.UUID, error)

type PhotoSource interface {
	ListByEvent(ctx context.Context, in photos.ListInput) ([]*photos.Photo, error)
}

// Store is the streaming subset of object storage the ZIP worker needs.
type Store interface {
	GetReader(ctx context.Context, key string) (io.ReadCloser, error)
	PutReader(ctx context.Context, key, contentType string, r io.Reader, size int64) error
}

type Deleter interface {
	Delete(ctx context.Context, key string) error
}

// GenerateHandler builds a ZIP of an event's original uploads on a temp file
// and streams it to storage. It is safe to re-run: archives already ready or
// expired are skipped, and transient failures leave the export processing so
// the queue can retry. The archive is never built in memory.
func GenerateHandler(repo Repository, owner EventOwner, source PhotoSource, store Store, ttl time.Duration, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, payload []byte) error {
		var in GeneratePayload
		if err := json.Unmarshal(payload, &in); err != nil {
			return fmt.Errorf("decode generate payload: %w", err)
		}
		if in.ExportID == uuid.Nil {
			return errors.New("generate payload missing exportId")
		}

		export, err := repo.GetByID(ctx, in.ExportID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return nil
			}
			return fmt.Errorf("get export: %w", err)
		}
		if export.Status == StatusReady || export.Status == StatusExpired {
			return nil
		}

		if err := repo.MarkProcessing(ctx, export.ID); err != nil {
			return fmt.Errorf("mark processing: %w", err)
		}

		ownerID, err := owner(ctx, export.EventID)
		if err != nil {
			if errors.Is(err, photos.ErrNotFound) || errors.Is(err, ErrNotFound) {
				return nil
			}
			return fmt.Errorf("resolve event owner: %w", err)
		}

		key := r2.ExportKey(ownerID, export.EventID, export.ID)
		size, count, err := buildArchive(ctx, ownerID, export.EventID, key, source, store, log)
		if err != nil {
			return err
		}

		if err := repo.MarkReady(ctx, export.ID, key, size, time.Now().UTC().Add(ttl)); err != nil {
			return fmt.Errorf("mark ready: %w", err)
		}
		log.Info("export generated", "export_id", export.ID, "event_id", export.EventID,
			"photos", count, "bytes", size)
		return nil
	}
}

// CleanupHandler expires archives whose TTL has passed (deleting the stored
// object but keeping the row for polling) and fails exports stuck processing.
func CleanupHandler(repo Repository, store Deleter, log *slog.Logger) func(context.Context, []byte) error {
	return func(ctx context.Context, _ []byte) error {
		now := time.Now().UTC()

		expired := 0
		for {
			batch, err := repo.ListExpired(ctx, now, cleanupBatchSize)
			if err != nil {
				return fmt.Errorf("list expired exports: %w", err)
			}
			for _, export := range batch {
				if export.ObjectKey != nil && *export.ObjectKey != "" {
					if err := store.Delete(ctx, *export.ObjectKey); err != nil && !errors.Is(err, r2.ErrNotFound) {
						return fmt.Errorf("delete export object: %w", err)
					}
				}
				if err := repo.MarkExpired(ctx, export.ID); err != nil {
					return fmt.Errorf("mark expired: %w", err)
				}
				expired++
			}
			if len(batch) < cleanupBatchSize {
				break
			}
		}

		stale := 0
		for {
			batch, err := repo.ListStaleProcessing(ctx, now.Add(-staleProcessingTimeout), cleanupBatchSize)
			if err != nil {
				return fmt.Errorf("list stale exports: %w", err)
			}
			for _, export := range batch {
				if err := repo.MarkFailed(ctx, export.ID, "export timed out"); err != nil {
					return fmt.Errorf("mark failed: %w", err)
				}
				stale++
			}
			if len(batch) < cleanupBatchSize {
				break
			}
		}

		if expired > 0 || stale > 0 {
			log.Info("export cleanup", "expired", expired, "timed_out", stale)
		}
		return nil
	}
}

func buildArchive(ctx context.Context, ownerID, eventID uuid.UUID, key string, source PhotoSource, store Store, log *slog.Logger) (int64, int, error) {
	f, err := os.CreateTemp("", "cpd-export-*.zip")
	if err != nil {
		return 0, 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)
	defer f.Close()

	zw := zip.NewWriter(f)
	seen := map[string]int{}
	count := 0
	var cursor *photos.Cursor
	for {
		batch, err := source.ListByEvent(ctx, photos.ListInput{
			EventID: eventID,
			UserID:  ownerID,
			Cursor:  cursor,
			Status:  photos.StatusReady,
			Limit:   photoPageSize,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("list photos: %w", err)
		}
		for _, photo := range batch {
			if err := appendPhoto(ctx, zw, store, photo, uniqueEntryName(seen, entryName(photo)), log); err != nil {
				return 0, 0, err
			}
			count++
		}
		if len(batch) < photoPageSize {
			break
		}
		last := batch[len(batch)-1]
		cursor = &photos.Cursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	if err := zw.Close(); err != nil {
		return 0, 0, fmt.Errorf("close zip: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		return 0, 0, fmt.Errorf("stat archive: %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return 0, 0, fmt.Errorf("rewind archive: %w", err)
	}
	if err := store.PutReader(ctx, key, "application/zip", f, info.Size()); err != nil {
		return 0, 0, fmt.Errorf("upload archive: %w", err)
	}
	return info.Size(), count, nil
}

func appendPhoto(ctx context.Context, zw *zip.Writer, store Store, photo *photos.Photo, name string, log *slog.Logger) error {
	reader, err := store.GetReader(ctx, photo.StorageKey)
	if err != nil {
		if errors.Is(err, r2.ErrNotFound) {
			log.Warn("export skipping missing object", "photo_id", photo.ID, "key", photo.StorageKey)
			return nil
		}
		return fmt.Errorf("read object: %w", err)
	}
	defer reader.Close()

	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: photo.CreatedAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("add zip entry: %w", err)
	}
	if _, err := io.Copy(w, reader); err != nil {
		return fmt.Errorf("copy object: %w", err)
	}
	return nil
}

func entryName(photo *photos.Photo) string {
	name := ""
	if photo.OriginalFilename != nil {
		name = sanitizeEntryName(*photo.OriginalFilename)
	}
	if name != "" {
		return name
	}
	return "photo-" + photo.ID.String() + extensionForMime(photo.MimeType)
}

func sanitizeEntryName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.TrimSpace(name)
	name = strings.TrimLeft(name, ".")
	if name == "" || name == "/" {
		return ""
	}
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}

func extensionForMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

// uniqueEntryName dedupes case-insensitively, adding " (2)", " (3)", ... before
// the extension so two photos with the same filename do not collide in the ZIP.
func uniqueEntryName(seen map[string]int, name string) string {
	key := strings.ToLower(name)
	n := seen[key]
	seen[key] = n + 1
	if n == 0 {
		return name
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s (%d)%s", base, n+1, ext)
}
