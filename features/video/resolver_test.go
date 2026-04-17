package video

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
)

// mockAuthenticatedClient is a test double that records the requested URL
// and returns a preconfigured response.
type mockAuthenticatedClient struct {
	responseBody string
	responseErr  error
	lastURL      string
}

func (m *mockAuthenticatedClient) Get(_ context.Context, _ string) (*http.Response, error) {
	if m.responseErr != nil {
		return nil, m.responseErr
	}
	return nil, errors.New("Get not implemented in mock; use GetWithRetry")
}

func (m *mockAuthenticatedClient) GetWithRetry(_ context.Context, url string, _ int) ([]byte, error) {
	m.lastURL = url
	if m.responseErr != nil {
		return nil, m.responseErr
	}
	return []byte(m.responseBody), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func videoFixture(title, progressiveURL string, lines []transcriptLineData) videoAPIResponse {
	return videoAPIResponse{
		Elements: []videoElement{
			{
				SelectedVideo: &selectedVideoData{
					Title:             title,
					DurationInSeconds: 120,
					URL: &videoURLData{
						ProgressiveURL: progressiveURL,
					},
					Transcript: &transcriptData{
						Lines: lines,
					},
				},
			},
		},
	}
}

func csrfFixture() string {
	return `{"elements":[],"paging":{"start":0,"count":0,"total":0,"links":[]},"message":"CSRF check failed"}`
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLinkedInResolver_Resolve_Success(t *testing.T) {
	lines := []transcriptLineData{
		{Caption: "Hello world", TranscriptStartAt: 0},
		{Caption: "Goodbye world", TranscriptStartAt: 5000},
	}
	fixture := videoFixture("Test Video", "https://example.com/video.mp4", lines)
	body, _ := json.Marshal(fixture)

	client := &mockAuthenticatedClient{responseBody: string(body)}
	resolver := NewLinkedInResolver(client, shareddomain.QualityHigh)

	result, err := resolver.Resolve(context.Background(), "test-course", "test-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Title != "Test Video" {
		t.Errorf("Title = %q, want %q", result.Title, "Test Video")
	}
	if result.Slug != "test-video" {
		t.Errorf("Slug = %q, want %q", result.Slug, "test-video")
	}
	if result.Duration != 120 {
		t.Errorf("Duration = %d, want 120", result.Duration)
	}
	if result.DownloadURL != "https://example.com/video.mp4" {
		t.Errorf("DownloadURL = %q, want %q", result.DownloadURL, "https://example.com/video.mp4")
	}
	if len(result.TranscriptLines) != 2 {
		t.Fatalf("TranscriptLines len = %d, want 2", len(result.TranscriptLines))
	}
	if result.TranscriptLines[0].Caption != "Hello world" {
		t.Errorf("TranscriptLines[0].Caption = %q, want %q", result.TranscriptLines[0].Caption, "Hello world")
	}
	if result.TranscriptLines[0].StartsAt != 0 {
		t.Errorf("TranscriptLines[0].StartsAt = %d, want 0", result.TranscriptLines[0].StartsAt)
	}
	if result.TranscriptLines[1].StartsAt != 5000 {
		t.Errorf("TranscriptLines[1].StartsAt = %d, want 5000", result.TranscriptLines[1].StartsAt)
	}
}

func TestLinkedInResolver_Resolve_CSRFFailure(t *testing.T) {
	client := &mockAuthenticatedClient{responseBody: csrfFixture()}
	resolver := NewLinkedInResolver(client, shareddomain.QualityHigh)

	_, err := resolver.Resolve(context.Background(), "test-course", "test-video")
	if err == nil {
		t.Fatal("expected error for CSRF failure, got nil")
	}

	var authErr *sharederrors.AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected AuthError, got %T: %v", err, err)
	}
}

func TestLinkedInResolver_Resolve_MissingSelectedVideo(t *testing.T) {
	// Response with empty elements array.
	fixture := videoAPIResponse{
		Elements: []videoElement{},
		Paging:   pagingInfo{},
	}
	body, _ := json.Marshal(fixture)

	client := &mockAuthenticatedClient{responseBody: string(body)}
	resolver := NewLinkedInResolver(client, shareddomain.QualityHigh)

	_, err := resolver.Resolve(context.Background(), "test-course", "test-video")
	if err == nil {
		t.Fatal("expected error for missing selectedVideo, got nil")
	}

	var parseErr *sharederrors.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected ParseError, got %T: %v", err, err)
	}
	if !strings.Contains(parseErr.Error(), "selectedVideo") {
		t.Errorf("ParseError should mention selectedVideo, got: %v", parseErr)
	}
}

func TestLinkedInResolver_Resolve_MissingProgressiveURL(t *testing.T) {
	// SelectedVideo with nil URL field.
	fixture := videoAPIResponse{
		Elements: []videoElement{
			{
				SelectedVideo: &selectedVideoData{
					Title:             "No URL Video",
					DurationInSeconds: 60,
					URL:               nil,
				},
			},
		},
	}
	body, _ := json.Marshal(fixture)

	client := &mockAuthenticatedClient{responseBody: string(body)}
	resolver := NewLinkedInResolver(client, shareddomain.QualityHigh)

	_, err := resolver.Resolve(context.Background(), "test-course", "test-video")
	if err == nil {
		t.Fatal("expected error for missing progressiveUrl, got nil")
	}

	var parseErr *sharederrors.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("expected ParseError, got %T: %v", err, err)
	}
	if !strings.Contains(parseErr.Error(), "progressiveUrl") {
		t.Errorf("ParseError should mention progressiveUrl, got: %v", parseErr)
	}
}

func TestLinkedInResolver_InterfaceCompliance(_ *testing.T) {
	// Compile-time check is in linkedin.go (var _ Resolver = (*linkedinResolver)(nil)).
	// This is a no-op runtime placeholder confirming the test file compiles.
	_ = &linkedinResolver{}
}

func TestBuildVideoAPIURL(t *testing.T) {
	got := buildVideoAPIURL("my-course", "my-video", "720")
	if !strings.HasPrefix(got, "https://www.linkedin.com/learning-api/detailedCourses?") {
		t.Errorf("unexpected URL prefix: %s", got)
	}
	if !strings.Contains(got, "courseSlug=my-course") {
		t.Errorf("URL missing courseSlug: %s", got)
	}
	if !strings.Contains(got, "videoSlug=my-video") {
		t.Errorf("URL missing videoSlug: %s", got)
	}
	if !strings.Contains(got, "resolution=_720") {
		t.Errorf("URL missing resolution: %s", got)
	}
	if !strings.Contains(got, "fields=selectedVideo") {
		t.Errorf("URL missing fields: %s", got)
	}
}
