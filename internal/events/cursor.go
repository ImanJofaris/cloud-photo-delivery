package events

import (
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

var ErrInvalidCursor = errors.New("invalid cursor")

// Cursor is an opaque keyset position over (created_at, id).
type Cursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

func EncodeCursor(c Cursor) string {
	raw := c.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(s string) (*Cursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, ErrInvalidCursor
	}
	id, err := uuid.Parse(parts[1])
	if err != nil || id == uuid.Nil {
		return nil, ErrInvalidCursor
	}
	return &Cursor{CreatedAt: ts, ID: id}, nil
}

func NormalizeLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
