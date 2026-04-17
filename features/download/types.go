// Package download implements concurrent file downloading with worker pools,
// retry on transient failures, and progress tracking.
package download

// Job represents a single file download task.
// The app layer translates domain objects (Course, Video) into Jobs
// so this feature stays decoupled from other feature modules.
type Job struct {
	// URL is the remote file URL to download. Empty means write Content directly.
	URL string
	// DestPath is the absolute local file path to write.
	DestPath string
	// Description is a human-readable label shown in progress bars and logs.
	Description string
	// Critical controls failure handling. Critical failures propagate as errors;
	// non-critical failures (e.g. dead exercise CDNs) are marked Skipped instead.
	Critical bool
	// Content, when non-empty, is written directly to DestPath instead of fetching URL.
	Content string
}

// Result captures the outcome of a single Job execution.
type Result struct {
	// Description mirrors the Job description for identification.
	Description string
	// Err holds the error if the download failed (nil on success).
	Err error
	// Critical mirrors the Job's critical flag.
	Critical bool
	// Skipped is true when a non-critical job failed with a DNS error (dead CDN).
	Skipped bool
}
