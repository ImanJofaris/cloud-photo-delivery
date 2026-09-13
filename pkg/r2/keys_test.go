package r2

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSafeFilename_StripsPathTraversal(t *testing.T) {
	cases := map[string]string{
		"IMG_1234.jpg":          "IMG_1234.jpg",
		"../../etc/passwd":      "passwd",
		"/abs/path/photo.png":   "photo.png",
		`C:\Users\me\photo.jpg`: "photo.jpg",
		"....":                  "upload",
		"":                      "upload",
		"   ":                   "upload",
		".hidden":               "hidden",
	}
	for in, want := range cases {
		if got := SafeFilename(in); got != want {
			t.Errorf("SafeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOriginalKey_IsDeterministicAndScoped(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	eventID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	photoID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	got := OriginalKey(userID, eventID, photoID, "IMG_1234.jpg")
	want := "tenant/11111111-1111-1111-1111-111111111111/events/22222222-2222-2222-2222-222222222222/originals/33333333-3333-3333-3333-333333333333/IMG_1234.jpg"
	if got != want {
		t.Fatalf("OriginalKey = %q, want %q", got, want)
	}
	if again := OriginalKey(userID, eventID, photoID, "IMG_1234.jpg"); again != got {
		t.Fatal("OriginalKey is not deterministic")
	}
}

func TestOriginalKey_RejectsTraversal(t *testing.T) {
	userID := uuid.New()
	eventID := uuid.New()
	photoID := uuid.New()

	got := OriginalKey(userID, eventID, photoID, "../../../secrets.env")
	if strings.Contains(got, "..") {
		t.Fatalf("key still contains traversal: %q", got)
	}
	if !strings.HasSuffix(got, "/secrets.env") {
		t.Fatalf("key = %q, want suffix /secrets.env", got)
	}
}

func TestExportKey_IsScopedAndZipped(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	eventID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	exportID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	got := ExportKey(userID, eventID, exportID)
	want := "tenant/11111111-1111-1111-1111-111111111111/events/22222222-2222-2222-2222-222222222222/exports/44444444-4444-4444-4444-444444444444.zip"
	if got != want {
		t.Fatalf("ExportKey = %q, want %q", got, want)
	}
	if again := ExportKey(userID, eventID, exportID); again != got {
		t.Fatal("ExportKey is not deterministic")
	}
}

func TestDerivedKeys_PointAtWebpDerivatives(t *testing.T) {
	userID := uuid.New()
	eventID := uuid.New()
	photoID := uuid.New()

	for _, tc := range []struct {
		got  string
		kind string
	}{
		{OptimizedKey(userID, eventID, photoID), "optimized"},
		{MediumKey(userID, eventID, photoID), "medium"},
		{ThumbnailKey(userID, eventID, photoID), "thumbnails"},
	} {
		want := "tenant/" + userID.String() + "/events/" + eventID.String() + "/" + tc.kind + "/" + photoID.String() + ".webp"
		if tc.got != want {
			t.Errorf("%s key = %q, want %q", tc.kind, tc.got, want)
		}
	}
}
