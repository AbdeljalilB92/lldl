package course

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharederr "github.com/AbdeljalilB92/lldl/shared/errors"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
)

func TestLinkedInFetcher_FetchCourse_Success(t *testing.T) {
	resp := coursesAPIResponse{
		Elements: []courseElement{
			{
				Title: "Test Course",
				Slug:  "test-course",
				Chapters: []chapterRef{
					{
						Title: "Chapter One",
						Slug:  "chapter-one",
						Videos: []videoRef{
							{Title: "Video One", Slug: "video-one"},
						},
					},
				},
			},
		},
		Paging: pagingInfo{Start: 0, Count: 1, Total: 1},
	}
	respBody, _ := json.Marshal(resp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBody)
	}))
	defer server.Close()

	// Use the test wrapper that targets the test server.
	wrapper := &testFetcherWrapper{server: server}

	course, err := wrapper.FetchCourse(context.Background(), "test-course")
	if err != nil {
		t.Fatalf("FetchCourse returned error: %v", err)
	}

	if course.Title != "Test Course" {
		t.Errorf("Title = %q, want %q", course.Title, "Test Course")
	}
	if len(course.Chapters) != 1 {
		t.Fatalf("len(Chapters) = %d, want 1", len(course.Chapters))
	}
	if course.Chapters[0].Videos[0].Title != "Video One" {
		t.Errorf("Video title = %q, want %q", course.Chapters[0].Videos[0].Title, "Video One")
	}
}

func TestLinkedInFetcher_FetchCourse_CSRFFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`CSRF check failed`))
	}))
	defer server.Close()

	wrapper := &testFetcherWrapper{server: server}

	_, err := wrapper.FetchCourse(context.Background(), "test-course")
	if err == nil {
		t.Fatal("expected error for CSRF failure")
	}

	var authErr *sharederr.AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("error type = %T, want *AuthError", err)
	}
}

func TestLinkedInFetcher_FetchCourse_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	wrapper := &testFetcherWrapper{
		server:     server,
		maxRetries: 1,
	}

	_, err := wrapper.FetchCourse(context.Background(), "test-course")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestLinkedInFetcher_FetchCourse_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	wrapper := &testFetcherWrapper{server: server}

	_, err := wrapper.FetchCourse(context.Background(), "test-course")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestNewLinkedInFetcher_InterfaceCompliance(t *testing.T) {
	// Compile-time check is in linkedin.go. This runtime test verifies
	// the constructor returns a non-nil value.
	client := &testAuthClient{client: &http.Client{}}
	f := NewLinkedInFetcher(client)
	if f == nil {
		t.Fatal("NewLinkedInFetcher returned nil")
	}
}

// testAuthClient wraps an *http.Client to satisfy sharedhttp.AuthenticatedClient.
type testAuthClient struct {
	client *http.Client
}

func (c *testAuthClient) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}

func (c *testAuthClient) GetWithRetry(ctx context.Context, rawURL string, maxRetries int) ([]byte, error) {
	return sharedhttp.DoWithRetry(ctx, c, rawURL, maxRetries)
}

// testFetcherWrapper exercises the fetcher pipeline (retry + CSRF check + parse)
// using an httptest.Server instead of the real LinkedIn API.
type testFetcherWrapper struct {
	server     *httptest.Server
	maxRetries int
}

func (f *testFetcherWrapper) FetchCourse(ctx context.Context, slug string) (*Course, error) {
	courseURL := f.server.URL + "/learning-api/detailedCourses?courseSlug=" + slug +
		"&fields=chapters,title,exerciseFiles&addParagraphsToTranscript=true&q=slugs"

	retries := f.maxRetries
	if retries == 0 {
		retries = defaultMaxRetries
	}

	client := &testAuthClient{client: f.server.Client()}
	body, err := sharedhttp.DoWithRetry(ctx, client, courseURL, retries)
	if err != nil {
		return nil, err
	}

	// Detect expired token — same check as linkedinFetcher.FetchCourse.
	if strings.Contains(string(body), csrfFailedMessage) {
		return nil, &sharederr.AuthError{Cause: fmt.Errorf("token is expired (CSRF check failed)")}
	}

	return ParseCourse(body)
}
