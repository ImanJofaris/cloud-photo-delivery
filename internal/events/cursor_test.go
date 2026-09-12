package events

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCursor_RoundTrip(t *testing.T) {
	orig := Cursor{CreatedAt: time.Now().UTC().Truncate(time.Nanosecond), ID: uuid.New()}
	encoded := EncodeCursor(orig)
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.CreatedAt.Equal(orig.CreatedAt) {
		t.Fatalf("created_at %v != %v", decoded.CreatedAt, orig.CreatedAt)
	}
	if decoded.ID != orig.ID {
		t.Fatalf("id %v != %v", decoded.ID, orig.ID)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	c, err := DecodeCursor("")
	if err != nil || c != nil {
		t.Fatalf("expected nil,nil got %v,%v", c, err)
	}
}

func TestDecodeCursor_Malformed(t *testing.T) {
	cases := []string{
		"not-base64!!!",
		// base64 of "hello" with no separator
		"aGVsbG8",
		// base64 of "not-a-timestamp|not-a-uuid"
		"bm90LWEtdGltZXN0YW1wfG5vdC1hLXV1aWQ",
	}
	for _, c := range cases {
		if _, err := DecodeCursor(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestNormalizeLimit(t *testing.T) {
	if NormalizeLimit(0) != DefaultLimit {
		t.Fatal("zero should default")
	}
	if NormalizeLimit(-5) != DefaultLimit {
		t.Fatal("negative should default")
	}
	if NormalizeLimit(500) != MaxLimit {
		t.Fatal("over max should cap")
	}
	if NormalizeLimit(10) != 10 {
		t.Fatal("valid should pass through")
	}
}
