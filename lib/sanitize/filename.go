// Package sanitize provides pure filename sanitization utilities.
// It strips characters that are invalid or problematic in common filesystems.
package sanitize

import (
	"strings"
	"unicode"
)

// windowsReservedNames are filenames that are forbidden on Windows.
// Matching is case-insensitive. Prefixing with underscore avoids the conflict.
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true,
	"COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true,
	"LPT9": true,
}

// ToSafeFileName removes characters that are invalid in filenames across
// Windows, macOS, and Linux: < > : " / \ | ? * and all control characters.
// Spaces and Unicode are preserved so international titles remain readable.
//
// Windows reserved names (CON, PRN, AUX, NUL, COM1-9, LPT1-9) are prefixed
// with an underscore to avoid OS-level conflicts.
// Trailing spaces and periods are stripped for Windows compatibility.
// Returns "unnamed" when the result would be empty (e.g. all characters were stripped).
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
	result := sb.String()

	// Trim trailing spaces and periods for Windows compatibility.
	// Windows Explorer silently strips these, which can cause mismatches.
	result = strings.TrimRight(result, " .")

	if result == "" {
		return "unnamed"
	}

	// Prefix Windows reserved names with underscore to avoid OS conflicts.
	// Windows reserves device names even with extensions (CON.txt, NUL.log),
	// so we check the base name before the first dot.
	baseName := result
	if dotIdx := strings.Index(result, "."); dotIdx != -1 {
		baseName = result[:dotIdx]
	}
	if windowsReservedNames[strings.ToUpper(baseName)] {
		result = "_" + result
	}

	return result
}
