package events

import (
	"strings"
	"unicode"
)

const maxSlugLength = 255

// Slugify turns a free-form event name into an ASCII, lowercase, hyphenated
// slug. It returns an empty string when nothing usable remains.
func Slugify(name string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range name {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(unicode.ToLower(r))
			lastHyphen = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '.' || r == '\'' || r == '’':
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > maxSlugLength {
		slug = strings.Trim(slug[:maxSlugLength], "-")
	}
	return slug
}

// SuffixSlug appends a numeric collision suffix, e.g. "party" + 2 -> "party-2".
func SuffixSlug(slug string, n int) string {
	suffix := "-" + itoa(n)
	if len(slug)+len(suffix) > maxSlugLength {
		slug = strings.Trim(slug[:maxSlugLength-len(suffix)], "-")
	}
	return slug + suffix
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
