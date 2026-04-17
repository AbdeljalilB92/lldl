// Package sanitize provides pure filename sanitization utilities.
// It strips characters that are invalid or problematic in common filesystems.
package sanitize

import (
	"strings"
	"unicode"
)

// ToSafeFileName removes characters that are invalid in filenames across
// Windows, macOS, and Linux: < > : " / \ | ? * and all control characters.
// Spaces and Unicode are preserved so international titles remain readable.
func ToSafeFileName(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			continue
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
