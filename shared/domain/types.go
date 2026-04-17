// Package domain defines shared value objects used across multiple features.
// Types here are imported by features/ and shared/ but never import from features/ themselves.
package domain

import (
	"fmt"

	"github.com/AbdeljalilB92/lldl/shared/errors"
)

// Quality represents video quality as resolution height in pixels.
type Quality int

const (
	QualityHigh   Quality = 720
	QualityMedium Quality = 540
	QualityLow    Quality = 360
)

// String returns the resolution height as a plain string (e.g. "720").
func (q Quality) String() string {
	return fmt.Sprintf("%d", int(q))
}

// QualityFromString parses a quality value from numeric strings ("720", "540", "360"),
// numeric shorthand with "p" suffix ("720p", "540p", "360p"), menu indices ("1", "2", "3"),
// or friendly names ("high", "medium", "low").
// Returns a typed ValidationError on unrecognized input.
func QualityFromString(s string) (Quality, error) {
	switch s {
	case "720", "720p", "high", "1":
		return QualityHigh, nil
	case "540", "540p", "medium", "2":
		return QualityMedium, nil
	case "360", "360p", "low", "3":
		return QualityLow, nil
	default:
		return 0, &errors.ValidationError{
			Field:   "quality",
			Message: fmt.Sprintf("unknown quality %q (valid: 1, 2, 3, 720, 540, 360, high, medium, low)", s),
		}
	}
}

// TranscriptLine represents a single caption with its start time, used by both
// course and video features for transcript assembly.
type TranscriptLine struct {
	Caption  string `json:"caption"`
	StartsAt int64  `json:"startsAt"` // milliseconds
}

// FormatSRTTime converts milliseconds to SRT timestamp format: HH:MM:SS,mmm.
// Shared by course and video features to avoid duplicating SRT formatting logic.
func FormatSRTTime(ms int64) string {
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	millis := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, millis)
}

// ExerciseFile represents a downloadable exercise resource attached to a course.
// Shared between course listing and exercise resolution features.
type ExerciseFile struct {
	FileName    string `json:"fileName"`
	DownloadURL string `json:"downloadUrl"`
	FileSize    int64  `json:"fileSize"`
}

// SafeFileName is a string wrapper representing a filesystem-safe filename.
// Values should be produced by lib/sanitize before being assigned.
type SafeFileName string
