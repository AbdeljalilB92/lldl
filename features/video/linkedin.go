package video

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
)

// Compile-time check: linkedinResolver satisfies the Resolver interface.
var _ Resolver = (*linkedinResolver)(nil)

// linkedinResolver resolves video stream URLs and transcripts via the
// LinkedIn Learning detailedCourses API.
type linkedinResolver struct {
	client  sharedhttp.AuthenticatedClient
	quality shareddomain.Quality
}

// NewLinkedInResolver creates a Resolver backed by the LinkedIn Learning API.
// The quality parameter selects the video resolution (720, 540, or 360).
func NewLinkedInResolver(client sharedhttp.AuthenticatedClient, quality shareddomain.Quality) Resolver {
	return &linkedinResolver{
		client:  client,
		quality: quality,
	}
}

// Resolve fetches the download URL and transcript for the video identified by
// videoSlug within courseSlug. It returns a fully populated Result.
func (r *linkedinResolver) Resolve(ctx context.Context, courseSlug string, videoSlug string) (*Result, error) {
	logger := logging.New("[Video][Resolve]")
	logger.Info("resolving video", "course", courseSlug, "video", videoSlug)

	apiURL := buildVideoAPIURL(courseSlug, videoSlug, r.quality.String())
	logger.Debug("API request", "url", apiURL)

	body, err := r.client.GetWithRetry(ctx, apiURL, 5)
	if err != nil {
		return nil, &sharederrors.NetworkError{URL: apiURL, Cause: err, Retryable: sharederrors.IsRetryable(err)}
	}

	if strings.Contains(string(body), "CSRF check failed") {
		return nil, &sharederrors.AuthError{
			Cause: fmt.Errorf("token is expired (CSRF check failed); please use a new li_at token"),
		}
	}

	var resp videoAPIResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, &sharederrors.ParseError{Source: "video API response", Cause: err}
	}

	if len(resp.Elements) == 0 || resp.Elements[0].SelectedVideo == nil {
		return nil, &sharederrors.ParseError{
			Source: "video API response",
			Cause:  fmt.Errorf("missing elements[0].selectedVideo"),
		}
	}

	sv := resp.Elements[0].SelectedVideo
	result := &Result{
		Title:    sv.Title,
		Slug:     videoSlug,
		Duration: sv.DurationInSeconds,
	}

	// Extract the progressive download URL.
	if sv.URL == nil || sv.URL.ProgressiveURL == "" {
		return nil, &sharederrors.ParseError{
			Source: "video API response",
			Cause:  fmt.Errorf("missing progressiveUrl in selectedVideo.url"),
		}
	}
	result.DownloadURL = sv.URL.ProgressiveURL

	// Extract transcript lines when available.
	if sv.Transcript != nil {
		for _, line := range sv.Transcript.Lines {
			result.TranscriptLines = append(result.TranscriptLines, shareddomain.TranscriptLine{
				Caption:  line.Caption,
				StartsAt: line.TranscriptStartAt,
			})
		}
	}

	logger.Info("video resolved", "title", sv.Title, "duration", sv.DurationInSeconds, "transcriptLines", len(result.TranscriptLines))

	return result, nil
}

// buildVideoAPIURL constructs the detailedCourses API URL using url.Values
// instead of string interpolation.
func buildVideoAPIURL(courseSlug, videoSlug, quality string) string {
	u := &url.URL{
		Scheme: "https",
		Host:   "www.linkedin.com",
		Path:   "/learning-api/detailedCourses",
	}
	q := u.Query()
	q.Set("courseSlug", courseSlug)
	q.Set("resolution", "_"+quality)
	q.Set("q", "slugs")
	q.Set("fields", "selectedVideo")
	q.Set("videoSlug", videoSlug)
	u.RawQuery = q.Encode()
	return u.String()
}
