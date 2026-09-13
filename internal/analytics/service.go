package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/imanjofaris/cloud-photo-delivery/internal/platform/apperr"
)

const (
	DefaultDays = 30
	MaxDays     = 365
)

type Service struct {
	repo   Repository
	events EventChecker
	salt   string
	log    *slog.Logger
	now    func() time.Time
}

func NewService(repo Repository, events EventChecker, salt string, log *slog.Logger) *Service {
	return &Service{repo: repo, events: events, salt: salt, log: log, now: time.Now}
}

func (s *Service) SetClock(now func() time.Time) { s.now = now }

// RecordView is called by the public gallery on every successful view. It is
// best-effort by contract: callers ignore the error so a counter write can
// never fail a gallery read.
func (s *Service) RecordView(ctx context.Context, eventID uuid.UUID, ip, userAgent string, qrScan bool) error {
	err := s.repo.RecordView(ctx, eventID, s.today(), hashVisitor(s.salt, ip, userAgent), qrScan)
	if err != nil {
		s.logError("analytics view record failed", eventID, err)
	}
	return err
}

func (s *Service) RecordDownload(ctx context.Context, eventID uuid.UUID) error {
	err := s.repo.RecordDownload(ctx, eventID, s.today())
	if err != nil {
		s.logError("analytics download record failed", eventID, err)
	}
	return err
}

func (s *Service) Event(ctx context.Context, userID, eventID uuid.UUID, days int) (*EventReport, error) {
	window, err := s.window(days)
	if err != nil {
		return nil, err
	}
	owned, err := s.events.EventOwnedBy(ctx, userID, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	if !owned {
		return nil, eventNotFound()
	}
	report, err := s.repo.EventSummary(ctx, eventID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	daily, err := s.repo.EventDaily(ctx, eventID, window)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	report.Daily = daily
	return &report, nil
}

func (s *Service) Account(ctx context.Context, userID uuid.UUID, days int) (*AccountReport, error) {
	window, err := s.window(days)
	if err != nil {
		return nil, err
	}
	report, err := s.repo.AccountSummary(ctx, userID)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	daily, err := s.repo.AccountDaily(ctx, userID, window)
	if err != nil {
		return nil, apperr.Internal().WithCause(err)
	}
	report.Daily = daily
	return &report, nil
}

// window validates the requested day count and returns the inclusive UTC start
// day so `days=1` means today only.
func (s *Service) window(days int) (time.Time, error) {
	if days == 0 {
		days = DefaultDays
	}
	if days < 1 || days > MaxDays {
		return time.Time{}, validationError("days must be between 1 and 365")
	}
	return s.today().AddDate(0, 0, -(days - 1)), nil
}

func (s *Service) today() time.Time {
	t := s.now().UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) logError(msg string, eventID uuid.UUID, err error) {
	if s.log == nil {
		return
	}
	s.log.Error(msg, "event_id", eventID, "error", err)
}

func hashVisitor(salt, ip, userAgent string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(ip))
	mac.Write([]byte{0})
	mac.Write([]byte(userAgent))
	return hex.EncodeToString(mac.Sum(nil))
}

func eventNotFound() *apperr.Error {
	return apperr.New("EVENT_NOT_FOUND", "Event not found", 404)
}

func validationError(msg string) *apperr.Error {
	return apperr.New("VALIDATION_ERROR", msg, 422)
}
