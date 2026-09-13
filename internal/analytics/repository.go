package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func scanCounters(row pgx.Row) (Counters, error) {
	var c Counters
	err := row.Scan(&c.GalleryViews, &c.UniqueVisitors, &c.Downloads, &c.QRScans)
	return c, err
}

// RecordView increments the gallery view counter, records the visitor for the
// day (incrementing unique visitors only the first time that hash is seen), and
// increments QR scans when requested. The transaction keeps the visitor row and
// the unique counter consistent on a crash.
func (r *PostgresRepository) RecordView(ctx context.Context, eventID uuid.UUID, day time.Time, visitorHash string, qrScan bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qr := 0
	if qrScan {
		qr = 1
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO event_analytics (event_id, day, gallery_views, qr_scans)
		 VALUES ($1, $2, 1, $3)
		 ON CONFLICT (event_id, day) DO UPDATE SET
			gallery_views = event_analytics.gallery_views + 1,
			qr_scans = event_analytics.qr_scans + EXCLUDED.qr_scans`,
		eventID, day, qr); err != nil {
		return err
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO event_visitors (event_id, day, visitor_hash)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		eventID, day, visitorHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE event_analytics SET unique_visitors = unique_visitors + 1
			 WHERE event_id = $1 AND day = $2`, eventID, day); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *PostgresRepository) RecordDownload(ctx context.Context, eventID uuid.UUID, day time.Time) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO event_analytics (event_id, day, downloads)
		 VALUES ($1, $2, 1)
		 ON CONFLICT (event_id, day) DO UPDATE SET
			downloads = event_analytics.downloads + 1`,
		eventID, day)
	return err
}

func (r *PostgresRepository) EventSummary(ctx context.Context, eventID uuid.UUID) (EventReport, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(a.gallery_views), 0), COALESCE(SUM(a.unique_visitors), 0),
			COALESCE(SUM(a.downloads), 0), COALESCE(SUM(a.qr_scans), 0),
			COALESCE((SELECT photo_count FROM events WHERE id = $1), 0)
		 FROM event_analytics a WHERE a.event_id = $1`, eventID)
	report := EventReport{EventID: eventID}
	err := row.Scan(&report.Totals.GalleryViews, &report.Totals.UniqueVisitors,
		&report.Totals.Downloads, &report.Totals.QRScans, &report.PhotoCount)
	return report, err
}

func (r *PostgresRepository) EventDaily(ctx context.Context, eventID uuid.UUID, since time.Time) ([]DayCounters, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT day, gallery_views, unique_visitors, downloads, qr_scans
		 FROM event_analytics
		 WHERE event_id = $1 AND day >= $2
		 ORDER BY day`, eventID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDaily(rows)
}

func (r *PostgresRepository) AccountSummary(ctx context.Context, userID uuid.UUID) (AccountReport, error) {
	report := AccountReport{}
	row := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(a.gallery_views), 0), COALESCE(SUM(a.unique_visitors), 0),
			COALESCE(SUM(a.downloads), 0), COALESCE(SUM(a.qr_scans), 0)
		 FROM event_analytics a
		 JOIN events e ON e.id = a.event_id
		 WHERE e.user_id = $1 AND e.deleted_at IS NULL`, userID)
	if err := row.Scan(&report.Totals.GalleryViews, &report.Totals.UniqueVisitors,
		&report.Totals.Downloads, &report.Totals.QRScans); err != nil {
		return AccountReport{}, err
	}

	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(photo_count), 0) FROM events
		 WHERE user_id = $1 AND deleted_at IS NULL`, userID).Scan(&report.EventCount, &report.PhotoCount); err != nil {
		return AccountReport{}, err
	}
	return report, nil
}

func (r *PostgresRepository) AccountDaily(ctx context.Context, userID uuid.UUID, since time.Time) ([]DayCounters, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.day, SUM(a.gallery_views), SUM(a.unique_visitors), SUM(a.downloads), SUM(a.qr_scans)
		 FROM event_analytics a
		 JOIN events e ON e.id = a.event_id
		 WHERE e.user_id = $1 AND e.deleted_at IS NULL AND a.day >= $2
		 GROUP BY a.day
		 ORDER BY a.day`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDaily(rows)
}

func scanDaily(rows pgx.Rows) ([]DayCounters, error) {
	var out []DayCounters
	for rows.Next() {
		var d DayCounters
		if err := rows.Scan(&d.Day, &d.GalleryViews, &d.UniqueVisitors, &d.Downloads, &d.QRScans); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
