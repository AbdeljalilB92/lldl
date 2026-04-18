package app

// Wails request/response types for GUI bindings.
// All structs are JSON-serializable for the Wails frontend.

// ConfigResponse is returned by LoadConfig so the frontend can pre-fill forms.
type ConfigResponse struct {
	Token     string `json:"token"`
	Quality   string `json:"quality"`
	OutputDir string `json:"outputDir"`
	CourseURL string `json:"courseUrl"`
	Found     bool   `json:"found"`
}

// SaveConfigRequest is sent by the frontend to persist user settings.
type SaveConfigRequest struct {
	Token     string `json:"token"`
	Quality   string `json:"quality"`
	OutputDir string `json:"outputDir"`
	CourseURL string `json:"courseUrl"`
}

// AuthResponse is returned by Authenticate to indicate success or failure.
type AuthResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ChapterResponse describes a single chapter with its videos for the frontend.
type ChapterResponse struct {
	Title  string          `json:"title"`
	Slug   string          `json:"slug"`
	Videos []VideoResponse `json:"videos"`
}

// VideoResponse describes a single video within a chapter.
type VideoResponse struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Duration    int    `json:"duration"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// CourseResponse is returned by FetchCourse with the full course structure.
type CourseResponse struct {
	Title        string            `json:"title"`
	ChapterCount int               `json:"chapterCount"`
	VideoCount   int               `json:"videoCount"`
	Chapters     []ChapterResponse `json:"chapters"`
	HasExercises bool              `json:"hasExercises"`
	Error        string            `json:"error,omitempty"`
}

// ResolveProgress is emitted as a Wails event during video/exercise resolution.
type ResolveProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Title   string `json:"title"`
	Error   string `json:"error,omitempty"`
}

// DownloadStartResponse is emitted when downloads begin.
type DownloadStartResponse struct {
	TotalJobs int `json:"totalJobs"`
}

// DownloadProgressEvent is emitted during each download job.
type DownloadProgressEvent struct {
	JobID      string `json:"jobId"`
	FileName   string `json:"fileName"`
	Status     string `json:"status"`
	BytesDone  int64  `json:"bytesDone"`
	BytesTotal int64  `json:"bytesTotal"`
	Error      string `json:"error,omitempty"`
}

// DownloadCompleteResponse is emitted when all downloads finish.
type DownloadCompleteResponse struct {
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	Skipped   int    `json:"skipped"`
	Error     string `json:"error,omitempty"`
}

// ErrorResponse is a generic error payload for Wails bindings.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
