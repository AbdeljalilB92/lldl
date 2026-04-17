package download

import (
	"fmt"

	"github.com/AbdeljalilB92/lldl/lib/sanitize"
)

// FormatChapterDir returns a formatted directory name for a chapter.
// Example: FormatChapterDir("Introduction", 0) returns "01 - Introduction".
func FormatChapterDir(title string, index int) string {
	safe := sanitize.ToSafeFileName(title)
	return fmt.Sprintf("%02d - %s", index+1, safe)
}

// FormatVideoFile returns a formatted filename for a video.
// Example: FormatVideoFile("Getting Started", "mp4") returns "Getting Started.mp4".
func FormatVideoFile(title string, ext string) string {
	safe := sanitize.ToSafeFileName(title)
	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}
	return safe + ext
}

// FormatVideoFileWithIndex returns a formatted filename for a video with its chapter-local index.
// Example: FormatVideoFileWithIndex("Getting Started", 0, "mp4") returns "01 - Getting Started.mp4".
func FormatVideoFileWithIndex(title string, index int, ext string) string {
	safe := sanitize.ToSafeFileName(title)
	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}
	return fmt.Sprintf("%02d - %s%s", index+1, safe, ext)
}
