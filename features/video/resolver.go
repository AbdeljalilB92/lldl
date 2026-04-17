package video

import "context"

// Resolver resolves a video's download URL and transcript from the LinkedIn API.
type Resolver interface {
	// Resolve fetches the stream URL and transcript for a single video
	// identified by its course and video slugs.
	Resolve(ctx context.Context, courseSlug string, videoSlug string) (*Result, error)
}
