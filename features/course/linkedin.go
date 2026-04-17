package course

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	sharederr "github.com/AbdeljalilB92/lldl/shared/errors"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
)

const (
	// defaultMaxRetries is the number of retry attempts for transient HTTP failures.
	defaultMaxRetries = 5

	// csrfFailedMessage is the substring that LinkedIn returns when the auth token
	// has expired and CSRF validation fails.
	csrfFailedMessage = "CSRF check failed"
)

// Compile-time check: linkedinFetcher satisfies the Fetcher interface.
var _ Fetcher = (*linkedinFetcher)(nil)

// linkedinFetcher fetches course structure from the LinkedIn Learning API.
// It only parses course metadata (chapters, titles, slugs) — video download URLs
// and transcripts are resolved separately by the video feature.
type linkedinFetcher struct {
	client sharedhttp.AuthenticatedClient
}

// NewLinkedInFetcher creates a Fetcher that retrieves course structure from the
// LinkedIn Learning detailedCourses API endpoint.
func NewLinkedInFetcher(client sharedhttp.AuthenticatedClient) Fetcher {
	return &linkedinFetcher{
		client: client,
	}
}

// FetchCourse retrieves the course structure for the given slug.
// It builds the API URL, fetches with retry, and delegates JSON parsing.
func (f *linkedinFetcher) FetchCourse(ctx context.Context, slug string) (*Course, error) {
	logger := logging.New("[Course][Fetcher]")

	courseURL := buildCourseAPIURL(slug)

	body, err := sharedhttp.DoWithRetry(ctx, f.client, courseURL, defaultMaxRetries)
	if err != nil {
		return nil, err
	}

	// Detect expired token before attempting to parse.
	if strings.Contains(string(body), csrfFailedMessage) {
		return nil, &sharederr.AuthError{Cause: fmt.Errorf("token is expired (CSRF check failed); please use a new li_at token")}
	}

	logger.Info("fetched course structure", "slug", slug)
	return ParseCourse(body)
}

// buildCourseAPIURL constructs the detailedCourses API URL using url.URL
// construction — no string interpolation into URLs.
func buildCourseAPIURL(slug string) string {
	u := &url.URL{
		Scheme: "https",
		Host:   "www.linkedin.com",
		Path:   "/learning-api/detailedCourses",
	}
	q := u.Query()
	q.Set("courseSlug", slug)
	q.Set("fields", "chapters,title,exerciseFiles")
	q.Set("addParagraphsToTranscript", "true")
	q.Set("q", "slugs")
	u.RawQuery = q.Encode()
	return u.String()
}
