package r2

import (
	"path"
	"strings"

	"github.com/google/uuid"
)

func OriginalKey(userID, eventID, photoID uuid.UUID, filename string) string {
	return path.Join(
		"tenant", userID.String(),
		"events", eventID.String(),
		"originals", photoID.String(),
		SafeFilename(filename),
	)
}

func derivedKey(userID, eventID, photoID uuid.UUID, kind string) string {
	return path.Join(
		"tenant", userID.String(),
		"events", eventID.String(),
		kind, photoID.String()+".webp",
	)
}

func OptimizedKey(userID, eventID, photoID uuid.UUID) string {
	return derivedKey(userID, eventID, photoID, "optimized")
}

func MediumKey(userID, eventID, photoID uuid.UUID) string {
	return derivedKey(userID, eventID, photoID, "medium")
}

func ThumbnailKey(userID, eventID, photoID uuid.UUID) string {
	return derivedKey(userID, eventID, photoID, "thumbnails")
}

// SafeFilename strips any path components from a client-supplied filename so a
// crafted name like "../../etc/passwd" cannot escape the photo prefix.
func SafeFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.TrimSpace(name)
	name = strings.TrimLeft(name, ".")
	if name == "" || name == "/" {
		return "upload"
	}
	if len(name) > 200 {
		name = name[len(name)-200:]
	}
	return name
}
