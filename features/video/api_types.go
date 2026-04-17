package video

// videoAPIResponse is the top-level shape of the LinkedIn detailedCourses API
// JSON when requesting video details (fields=selectedVideo).
type videoAPIResponse struct {
	Elements []videoElement `json:"elements"`
	Paging   pagingInfo     `json:"paging"`
}

// pagingInfo carries pagination metadata from the API response.
type pagingInfo struct {
	Start int `json:"start"`
	Count int `json:"count"`
	Total int `json:"total"`
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

// videoElement represents a single course element within the API response.
// The SelectedVideo field is populated when fields=selectedVideo is requested.
type videoElement struct {
	SelectedVideo *selectedVideoData `json:"selectedVideo"`
}

// selectedVideoData contains the resolved video metadata including the stream
// URL and transcript lines returned by the LinkedIn API.
type selectedVideoData struct {
	Title             string          `json:"title"`
	DurationInSeconds int             `json:"durationInSeconds"`
	URL               *videoURLData   `json:"url"`
	Transcript        *transcriptData `json:"transcript"`
}

// videoURLData wraps the progressive download URL for the video stream.
// The ProgressiveUrl field carries the direct MP4 link at the requested quality.
type videoURLData struct {
	ProgressiveURL string `json:"progressiveUrl"`
}

// transcriptData holds the caption lines for a video.
type transcriptData struct {
	Lines []transcriptLineData `json:"lines"`
}

// transcriptLineData represents one caption entry with its text and start time.
type transcriptLineData struct {
	Caption           string `json:"caption"`
	TranscriptStartAt int64  `json:"transcriptStartAt"`
}
