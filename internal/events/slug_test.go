package events

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "Summer Party", "summer-party"},
		{"mixed case", "IMAN's Wedding 2026", "iman-s-wedding-2026"},
		{"punctuation", "Annual Gala!!!", "annual-gala"},
		{"leading trailing", "  --Hello-- ", "hello"},
		{"collapses separators", "a   b___c", "a-b-c"},
		{"non ascii dropped", "Café Münich", "caf-mnich"},
		{"empty", "!!!", ""},
		{"only spaces", "   ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slugify(tt.in); got != tt.want {
				t.Fatalf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSuffixSlug(t *testing.T) {
	if got := SuffixSlug("party", 2); got != "party-2" {
		t.Fatalf("got %q", got)
	}
	if got := SuffixSlug("party", 10); got != "party-10" {
		t.Fatalf("got %q", got)
	}
}

func TestSuffixSlug_TruncatesLongSlug(t *testing.T) {
	long := make([]byte, 300)
	for i := range long {
		long[i] = 'a'
	}
	base := Slugify(string(long))
	got := SuffixSlug(base, 2)
	if len(got) > maxSlugLength {
		t.Fatalf("slug too long: %d", len(got))
	}
	if got[len(got)-2:] != "-2" {
		t.Fatalf("missing suffix: %q", got)
	}
}
