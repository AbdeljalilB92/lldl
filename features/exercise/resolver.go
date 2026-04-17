package exercise

import "context"

// Resolver resolves exercise file download URLs by scraping the course page
// HTML for embedded BPR (Big Pipe Render) JSON data.
type Resolver interface {
	// Resolve fetches the course page and extracts exercise file URLs.
	// firstVideoSlug is used to construct the page URL; enterprise users
	// may receive additional query parameters via the constructor.
	Resolve(ctx context.Context, courseSlug string, firstVideoSlug string) (*Result, error)
}
