// Package video defines the interface and logic for resolving video stream URLs
// and extracting transcripts from the LinkedIn Learning API. It imports only
// from shared/ and lib/ — no cross-feature imports.
package video

import shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"

// Result holds the resolved data for a single video: its download URL,
// metadata, and transcript lines. SRT formatting is done by the caller
// via course.Video.FormTranscript().
type Result struct {
	Title           string
	Slug            string
	Duration        int // seconds
	DownloadURL     string
	TranscriptLines []shareddomain.TranscriptLine
}

// StreamInfo describes a single video stream with its URL and quality label.
type StreamInfo struct {
	URL     string
	Quality string
}
