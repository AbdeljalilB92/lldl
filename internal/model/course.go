package model

import (
	"fmt"
	"strings"
	"unicode"
)

// Quality represents video quality resolution height.
type Quality int

const (
	QualityHigh   Quality = 720
	QualityMedium Quality = 540
	QualityLow    Quality = 360
)

// String returns the height string for the quality (e.g. "720").
func (q Quality) String() string {
	return fmt.Sprintf("%d", int(q))
}

// QualityFromString parses a quality value from strings like "720", "720p", "high", "540", "medium", "360", "low".
func QualityFromString(s string) (Quality, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, "p")

	switch s {
	case "1", "720", "high":
		return QualityHigh, nil
	case "2", "540", "medium":
		return QualityMedium, nil
	case "3", "360", "low":
		return QualityLow, nil
	default:
		return 0, fmt.Errorf("unknown quality: %q (valid: 720, 540, 360, high, medium, low)", s)
	}
}

// Course represents a LinkedIn Learning course.
type Course struct {
	Title         string         `json:"title"`
	Slug          string         `json:"slug"`
	Chapters      []Chapter      `json:"chapters"`
	ExerciseFiles []ExerciseFile `json:"exerciseFiles"`
}

// Chapter represents a course chapter containing videos.
type Chapter struct {
	Title         string  `json:"title"`
	Slug          string  `json:"slug"`
	Videos        []Video `json:"videos"`
	IndexInCourse int     `json:"indexInCourse"`
}

// Video represents a single video within a chapter.
type Video struct {
	Title           string           `json:"title"`
	Slug            string           `json:"slug"`
	Duration        int              `json:"duration"` // seconds
	DownloadURL     string           `json:"downloadUrl"`
	TranscriptLines []TranscriptLine `json:"transcriptLines"`
	Transcript      string           `json:"transcript"` // populated after FormTranscript
}

// FormTranscript builds an SRT-format transcript from TranscriptLines and sets Transcript.
func (v *Video) FormTranscript() {
	if v.TranscriptLines == nil {
		return
	}
	var sb strings.Builder
	for i, line := range v.TranscriptLines {
		startsAt := formatSRTTime(line.StartsAt)
		endsAtMS := line.StartsAt
		if i+1 < len(v.TranscriptLines) {
			endsAtMS = v.TranscriptLines[i+1].StartsAt
		} else {
			endsAtMS = int64(v.Duration) * 1000
		}
		endsAt := formatSRTTime(endsAtMS)

		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", startsAt, endsAt))
		sb.WriteString(line.Caption)
		sb.WriteString("\n\n")
	}
	v.Transcript = strings.TrimSpace(sb.String())
}

// formatSRTTime converts milliseconds to SRT time format: hh:mm:ss,fff
func formatSRTTime(ms int64) string {
	h := ms / 3600000
	ms %= 3600000
	m := ms / 60000
	ms %= 60000
	s := ms / 1000
	ms %= 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

// TranscriptLine represents a single line of video transcript with timing.
type TranscriptLine struct {
	Caption  string `json:"caption"`
	StartsAt int64  `json:"transcriptStartAt"` // milliseconds
}

// ExerciseFile represents a downloadable exercise file for a course.
type ExerciseFile struct {
	FileName    string `json:"fileName"`
	DownloadURL string `json:"downloadUrl"`
	FileSize    int64  `json:"fileSize"`
}

// ToSafeFileName strips characters that are invalid in filenames (< > : " / \ | ? * and control chars).
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

// FormatChapterDir returns a formatted directory name for a chapter, e.g. "01 - Introduction".
func FormatChapterDir(chapter Chapter) string {
	title := ToSafeFileName(chapter.Title)
	return fmt.Sprintf("%02d - %s", chapter.IndexInCourse+1, title)
}

// FormatVideoFile returns a formatted filename for a video, e.g. "Getting Started.mp4".
func FormatVideoFile(video Video, ext string) string {
	title := ToSafeFileName(video.Title)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return title + ext
}

// FormatVideoFileWithIndex returns a formatted filename for a video with its index, e.g. "01 - Getting Started.mp4".
func FormatVideoFileWithIndex(video Video, index int, ext string) string {
	title := ToSafeFileName(video.Title)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%02d - %s%s", index+1, title, ext)
}
