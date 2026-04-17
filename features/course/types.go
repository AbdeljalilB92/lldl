// Package course defines the domain types and logic for fetching and parsing
// LinkedIn Learning course data. It imports only from shared/ and lib/.
package course

import (
	"fmt"
	"strings"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
)

// Course represents a full LinkedIn Learning course with its chapters,
// videos, and optional exercise files.
type Course struct {
	Title         string
	Slug          string
	Chapters      []Chapter
	ExerciseFiles []shareddomain.ExerciseFile
}

// Chapter is a named section within a course, containing an ordered list of videos.
type Chapter struct {
	Title         string
	Slug          string
	Videos        []Video
	IndexInCourse int
}

// Video represents a single video within a chapter. DownloadURL and transcript
// fields are populated later by the video feature — this struct only carries
// the structural metadata parsed from the course API.
type Video struct {
	Title           string
	Slug            string
	Duration        int // seconds
	DownloadURL     string
	TranscriptLines []shareddomain.TranscriptLine
	Transcript      string
}

// FormTranscript builds an SRT-formatted transcript string from TranscriptLines.
// End timestamps are derived from the next line's start; the final line falls back
// to the video duration. The result is stored in v.Transcript.
func (v *Video) FormTranscript() {
	if len(v.TranscriptLines) == 0 {
		return
	}

	var b strings.Builder
	b.Grow(len(v.TranscriptLines) * 80)

	for i, line := range v.TranscriptLines {
		startsAt := shareddomain.FormatSRTTime(line.StartsAt)

		// End time: next line's start, or video duration for the last line.
		var endsAtMS int64
		if i+1 < len(v.TranscriptLines) {
			endsAtMS = v.TranscriptLines[i+1].StartsAt
		} else {
			endsAtMS = int64(v.Duration) * 1000
		}
		endsAt := shareddomain.FormatSRTTime(endsAtMS)

		fmt.Fprintf(&b, "%d\n", i+1)
		fmt.Fprintf(&b, "%s --> %s\n", startsAt, endsAt)
		fmt.Fprintf(&b, "%s\n\n", line.Caption)
	}

	v.Transcript = strings.TrimSpace(b.String())
}
