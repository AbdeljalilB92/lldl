package download

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
)

// mockClient implements sharedhttp.AuthenticatedClient for testing.
// It is defined here instead of importing a shared mock to keep test files self-contained.
type mockClient struct {
	getFn      func(ctx context.Context, _ string) (*http.Response, error)
	getRetryFn func(ctx context.Context, _ string, maxRetries int) ([]byte, error)
}

func (m *mockClient) Get(ctx context.Context, url string) (*http.Response, error) {
	if m.getFn != nil {
		return m.getFn(ctx, url)
	}
	return nil, fmt.Errorf("unexpected Get call")
}

func (m *mockClient) GetWithRetry(ctx context.Context, url string, maxRetries int) ([]byte, error) {
	if m.getRetryFn != nil {
		return m.getRetryFn(ctx, url, maxRetries)
	}
	return nil, fmt.Errorf("unexpected GetWithRetry call")
}

// --- Interface compliance ---

func TestConcurrentEngineImplementsEngine(t *testing.T) {
	// Compile-time check already exists in concurrent.go.
	// This test ensures the constructor returns a non-nil Engine.
	engine := NewConcurrentEngine(2, &mockClient{})
	if engine == nil {
		t.Fatal("NewConcurrentEngine returned nil")
	}
}

// --- Single file download ---

func TestDownloadAll_SingleFile(t *testing.T) {
	content := "hello world"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Write([]byte(content))
	}))
	defer srv.Close()

	dir := t.TempDir()
	destPath := filepath.Join(dir, "test.txt")

	engine := NewConcurrentEngine(1, &mockClient{
		getFn: func(ctx context.Context, _ string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			return http.DefaultClient.Do(req)
		},
	})

	jobs := []Job{{
		URL:         srv.URL,
		DestPath:    destPath,
		Description: "test file",
		Critical:    true,
	}}

	results := engine.DownloadAll(context.Background(), jobs)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != content {
		t.Fatalf("expected %q, got %q", content, string(got))
	}
}

// --- Content write (non-URL) ---

func TestDownloadAll_ContentWrite(t *testing.T) {
	dir := t.TempDir()
	destPath := filepath.Join(dir, "sub.srt")

	engine := NewConcurrentEngine(1, &mockClient{})

	jobs := []Job{{
		DestPath:    destPath,
		Description: "subtitle",
		Critical:    true,
		Content:     []byte("1\n00:00:00,000 --> 00:00:01,000\nHello"),
	}}

	results := engine.DownloadAll(context.Background(), jobs)

	if results[0].Err != nil {
		t.Fatalf("unexpected error: %v", results[0].Err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != string(jobs[0].Content) {
		t.Fatalf("content mismatch")
	}
}

// --- Empty jobs ---

func TestDownloadAll_EmptyJobs(t *testing.T) {
	engine := NewConcurrentEngine(2, &mockClient{})
	results := engine.DownloadAll(context.Background(), nil)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

// --- Concurrent downloads ---

func TestDownloadAll_ConcurrentDownloads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	engine := NewConcurrentEngine(3, &mockClient{
		getFn: func(ctx context.Context, _ string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			return http.DefaultClient.Do(req)
		},
	})

	jobs := make([]Job, 5)
	for i := range jobs {
		jobs[i] = Job{
			URL:         srv.URL,
			DestPath:    filepath.Join(dir, fmt.Sprintf("file%d.txt", i)),
			Description: fmt.Sprintf("file %d", i),
			Critical:    true,
		}
	}

	results := engine.DownloadAll(context.Background(), jobs)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("job %d failed: %v", i, r.Err)
		}
	}
}

// --- Context cancellation ---

func TestDownloadAll_ContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-block // hang until unblocked
	}))
	defer srv.Close()
	defer close(block)

	engine := NewConcurrentEngine(1, &mockClient{
		getFn: func(ctx context.Context, _ string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			return http.DefaultClient.Do(req)
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	dir := t.TempDir()
	jobs := []Job{{
		URL:         srv.URL,
		DestPath:    filepath.Join(dir, "slow.txt"),
		Description: "slow file",
		Critical:    true,
	}}

	results := engine.DownloadAll(ctx, jobs)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
}

// --- Non-critical DNS skip ---

func TestDownloadAll_NonCriticalDNSSkip(t *testing.T) {
	dnsErr := &net.DNSError{Name: "dead.cdn.example.com", Server: "", IsTimeout: false, IsTemporary: false}

	callCount := 0
	engine := NewConcurrentEngine(1, &mockClient{
		getFn: func(_ context.Context, _ string) (*http.Response, error) {
			callCount++
			return nil, &sharederrors.NetworkError{
				URL:       "https://dead.cdn.example.com/file.zip",
				Cause:     dnsErr,
				Retryable: false,
			}
		},
	})

	dir := t.TempDir()
	jobs := []Job{{
		URL:         "https://dead.cdn.example.com/file.zip",
		DestPath:    filepath.Join(dir, "file.zip"),
		Description: "exercise file",
		Critical:    false,
	}}

	results := engine.DownloadAll(context.Background(), jobs)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Non-critical DNS failure should not propagate as an error.
	if results[0].Err != nil {
		t.Fatalf("non-critical DNS error should not propagate, got: %v", results[0].Err)
	}
	if !results[0].Skipped {
		t.Fatal("expected Skipped=true for non-critical DNS error")
	}
	// Should only be called once — DNS errors should not be retried for non-critical.
	if callCount != 1 {
		t.Fatalf("expected 1 HTTP call (no retry on DNS), got %d", callCount)
	}
}

// --- Critical DNS failure propagates ---

func TestDownloadAll_CriticalDNSFailure(t *testing.T) {
	dnsErr := &net.DNSError{Name: "dead.server.com", Server: "", IsTimeout: false, IsTemporary: false}

	engine := NewConcurrentEngine(1, &mockClient{
		getFn: func(_ context.Context, _ string) (*http.Response, error) {
			return nil, &sharederrors.NetworkError{
				URL:       "https://dead.server.com/video.mp4",
				Cause:     dnsErr,
				Retryable: false,
			}
		},
	})

	dir := t.TempDir()
	jobs := []Job{{
		URL:         "https://dead.server.com/video.mp4",
		DestPath:    filepath.Join(dir, "video.mp4"),
		Description: "critical video",
		Critical:    true,
	}}

	results := engine.DownloadAll(context.Background(), jobs)

	if results[0].Err == nil {
		t.Fatal("critical DNS failure should propagate as error")
	}
	if results[0].Skipped {
		t.Fatal("critical failure should not be marked Skipped")
	}
}

// --- Default concurrency ---

func TestNewConcurrentEngine_DefaultConcurrency(t *testing.T) {
	engine := NewConcurrentEngine(0, &mockClient{})
	ce, ok := engine.(*concurrentEngine)
	if !ok {
		t.Fatal("expected *concurrentEngine type")
	}
	if ce.concurrency != 4 {
		t.Fatalf("expected default concurrency 4, got %d", ce.concurrency)
	}
}

// --- Result order matches job order ---

func TestDownloadAll_ResultOrderMatchesJobOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "1")
		w.Write([]byte("x"))
	}))
	defer srv.Close()

	engine := NewConcurrentEngine(4, &mockClient{
		getFn: func(ctx context.Context, _ string) (*http.Response, error) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			return http.DefaultClient.Do(req)
		},
	})

	dir := t.TempDir()
	descriptions := make([]string, 10)
	for i := range descriptions {
		descriptions[i] = fmt.Sprintf("file-%d", i)
	}

	jobs := make([]Job, len(descriptions))
	for i, desc := range descriptions {
		jobs[i] = Job{
			URL:         srv.URL,
			DestPath:    filepath.Join(dir, fmt.Sprintf("f%d.txt", i)),
			Description: desc,
			Critical:    true,
		}
	}

	results := engine.DownloadAll(context.Background(), jobs)

	for i, r := range results {
		if r.Description != descriptions[i] {
			t.Errorf("result[%d] description = %q, want %q", i, r.Description, descriptions[i])
		}
	}
}

// --- Format functions ---

func TestFormatChapterDir(t *testing.T) {
	tests := []struct {
		title string
		index int
		want  string
	}{
		{"Introduction", 0, "01 - Introduction"},
		{"Advanced Go", 9, "10 - Advanced Go"},
		{"Go/Concurrency", 1, "02 - GoConcurrency"},
		{"Questions?", 2, "03 - Questions"},
	}
	for _, tt := range tests {
		got := FormatChapterDir(tt.title, tt.index)
		if got != tt.want {
			t.Errorf("FormatChapterDir(%q, %d) = %q, want %q", tt.title, tt.index, got, tt.want)
		}
	}
}

func TestFormatVideoFile(t *testing.T) {
	tests := []struct {
		title string
		ext   string
		want  string
	}{
		{"Getting Started", "mp4", "Getting Started.mp4"},
		{"Hello", ".srt", "Hello.srt"},
		{"Video: One", "mp4", "Video One.mp4"},
	}
	for _, tt := range tests {
		got := FormatVideoFile(tt.title, tt.ext)
		if got != tt.want {
			t.Errorf("FormatVideoFile(%q, %q) = %q, want %q", tt.title, tt.ext, got, tt.want)
		}
	}
}

func TestFormatVideoFileWithIndex(t *testing.T) {
	tests := []struct {
		title string
		index int
		ext   string
		want  string
	}{
		{"Getting Started", 0, "mp4", "01 - Getting Started.mp4"},
		{"Part Two", 1, ".srt", "02 - Part Two.srt"},
		{"Final", 9, "mp4", "10 - Final.mp4"},
	}
	for _, tt := range tests {
		got := FormatVideoFileWithIndex(tt.title, tt.index, tt.ext)
		if got != tt.want {
			t.Errorf("FormatVideoFileWithIndex(%q, %d, %q) = %q, want %q",
				tt.title, tt.index, tt.ext, got, tt.want)
		}
	}
}

// --- Helper: isHTTP404 ---

func TestIsHTTP404(t *testing.T) {
	tests := []struct {
		err  error
		want bool
	}{
		{fmt.Errorf("HTTP 404: resource not found"), true},
		{fmt.Errorf("HTTP 404"), true},
		{fmt.Errorf("not found"), true},
		{fmt.Errorf("HTTP 500: internal server error"), false},
		{nil, false},
	}
	for _, tt := range tests {
		got := isHTTP404(tt.err)
		if got != tt.want {
			t.Errorf("isHTTP404(%v) = %v, want %v", tt.err, got, tt.want)
		}
	}
}

// --- Helper: parentDir ---

func TestParentDir(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/tmp/file.txt", "/tmp"},
		{"/tmp/sub/file.txt", "/tmp/sub"},
		{"file.txt", ""},
		{"/", ""},
	}
	for _, tt := range tests {
		got := parentDir(tt.path)
		if got != tt.want {
			t.Errorf("parentDir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// Ensure the errors.Is chain works for NetworkError.
func TestDownloadAll_NetworkErrorIsRetryable(t *testing.T) {
	inner := errors.New("connection refused")
	wrapped := &sharederrors.NetworkError{URL: "http://example.com", Cause: inner, Retryable: true}

	if !sharederrors.IsRetryable(wrapped) {
		t.Fatal("NetworkError with Retryable=true should be retryable")
	}
	if sharederrors.IsDNSError(wrapped) {
		t.Fatal("generic network error should not be DNS error")
	}
}

// Verify that retry happens on transient (non-DNS, non-404) errors.
func TestDownloadWithRetry_RetriesOnTransientError(t *testing.T) {
	callCount := 0
	engine := NewConcurrentEngine(1, &mockClient{
		getFn: func(_ context.Context, _ string) (*http.Response, error) {
			callCount++
			// Fail first 2 times, succeed on 3rd.
			if callCount <= 2 {
				return nil, &sharederrors.NetworkError{
					URL:       "http://example.com/retry.mp4",
					Cause:     fmt.Errorf("connection reset"),
					Retryable: true,
				}
			}
			// Success on 3rd call.
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       http.NoBody,
				Header:     make(http.Header),
			}, nil
		},
	})

	dir := t.TempDir()
	jobs := []Job{{
		URL:         "http://example.com/retry.mp4",
		DestPath:    filepath.Join(dir, "retry.mp4"),
		Description: "retry file",
		Critical:    true,
	}}

	results := engine.DownloadAll(context.Background(), jobs)

	if callCount != 3 {
		t.Fatalf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
	if results[0].Err != nil {
		t.Fatalf("expected success after retries, got: %v", results[0].Err)
	}
}
