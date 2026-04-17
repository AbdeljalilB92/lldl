package course

import "context"

// Fetcher defines the contract for fetching a course's structural metadata
// (chapters, titles, slugs) from a backend. It does NOT resolve video download
// URLs — that is the video feature's responsibility.
type Fetcher interface {
	FetchCourse(ctx context.Context, slug string) (*Course, error)
}
