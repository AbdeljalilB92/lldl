package exercise

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// mockAuthenticatedClient is a test double that records the requested URL
// and returns a preconfigured HTTP response.
type mockAuthenticatedClient struct {
	responseBody string
	responseErr  error
	statusCode   int
	lastURL      string
}

func (m *mockAuthenticatedClient) Get(_ context.Context, url string) (*http.Response, error) {
	m.lastURL = url
	if m.responseErr != nil {
		return nil, m.responseErr
	}
	body := io.NopCloser(newStringReader(m.responseBody))
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       body,
	}, nil
}

func (m *mockAuthenticatedClient) GetWithRetry(_ context.Context, url string, _ int) ([]byte, error) {
	m.lastURL = url
	if m.responseErr != nil {
		return nil, m.responseErr
	}
	return []byte(m.responseBody), nil
}

type stringReader struct {
	s      string
	offset int
}

func newStringReader(s string) *stringReader { return &stringReader{s: s} }
func (r *stringReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.offset:])
	r.offset += n
	return n, nil
}

// buildTestHTML wraps a BPR JSON payload in a <code id="bpr-guid-xxx"> tag
// inside a minimal HTML page.
func buildTestHTML(bprJSON string) string {
	return `<!DOCTYPE html><html><body>` +
		`<code id="bpr-guid-abc123">` + htmlEscape(bprJSON) + `</code>` +
		`</body></html>`
}

func htmlEscape(s string) string {
	return html.EscapeString(s)
}

// exerciseBPR returns a BPR JSON payload with exercise file entries.
func exerciseBPR(files []exerciseEntry) string {
	bpr := bprData{
		Included: []bprIncludedItem{
			{
				DollarType:    "com.linkedin.learning.api.deco.content.Course",
				ExerciseFiles: files,
			},
		},
	}
	data, _ := json.Marshal(bpr)
	return string(data)
}

// noExerciseBPR returns a BPR JSON payload with no exercise files.
func noExerciseBPR() string {
	bpr := bprData{
		Included: []bprIncludedItem{
			{
				DollarType:    "com.linkedin.learning.api.deco.content.Course",
				ExerciseFiles: nil,
			},
		},
	}
	data, _ := json.Marshal(bpr)
	return string(data)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLinkedInResolver_Resolve_Success(t *testing.T) {
	files := []exerciseEntry{
		{Name: "exercise.zip", URL: "https://ambry.example.com/file1.zip", SizeInBytes: 1024},
		{Name: "starter-code.tar.gz", URL: "https://ambry.example.com/file2.tar.gz", SizeInBytes: 2048},
	}
	bprJSON := exerciseBPR(files)
	htmlBody := buildTestHTML(bprJSON)

	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 2 {
		t.Fatalf("Files len = %d, want 2", len(result.Files))
	}
	if result.Files[0].FileName != "exercise.zip" {
		t.Errorf("Files[0].FileName = %q, want %q", result.Files[0].FileName, "exercise.zip")
	}
	if result.Files[0].DownloadURL != "https://ambry.example.com/file1.zip" {
		t.Errorf("Files[0].DownloadURL = %q, want ambry URL", result.Files[0].DownloadURL)
	}
	if result.Files[0].FileSize != 1024 {
		t.Errorf("Files[0].FileSize = %d, want 1024", result.Files[0].FileSize)
	}
	if result.Files[1].FileName != "starter-code.tar.gz" {
		t.Errorf("Files[1].FileName = %q, want %q", result.Files[1].FileName, "starter-code.tar.gz")
	}
	if result.Files[1].FileSize != 2048 {
		t.Errorf("Files[1].FileSize = %d, want 2048", result.Files[1].FileSize)
	}
}

func TestLinkedInResolver_Resolve_NoExerciseFiles(t *testing.T) {
	htmlBody := buildTestHTML(noExerciseBPR())

	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("Files len = %d, want 0", len(result.Files))
	}
}

func TestLinkedInResolver_Resolve_NoBPRBlocks(t *testing.T) {
	htmlBody := "<html><body>No BPR blocks here</body></html>"

	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("Files len = %d, want 0 for page without BPR blocks", len(result.Files))
	}
}

func TestLinkedInResolver_Resolve_MalformedBPRJSON(t *testing.T) {
	// BPR block with invalid JSON — should be silently skipped, not returned as error.
	htmlBody := `<!DOCTYPE html><html><body>` +
		`<code id="bpr-guid-abc123">{not valid json!!!}</code>` +
		`</body></html>`

	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 0 {
		t.Errorf("Files len = %d, want 0 for malformed BPR JSON", len(result.Files))
	}
}

func TestLinkedInResolver_Resolve_HTTPError(t *testing.T) {
	client := &mockAuthenticatedClient{
		responseErr: &sharederrors.NetworkError{URL: "https://www.linkedin.com", Cause: io.EOF, Retryable: true},
	}
	resolver := NewLinkedInResolver(client, "")

	_, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err == nil {
		t.Fatal("expected error for HTTP failure, got nil")
	}
}

func TestLinkedInResolver_Resolve_NetworkFailure(t *testing.T) {
	client := &mockAuthenticatedClient{
		responseErr: &sharederrors.NetworkError{URL: "https://www.linkedin.com", Cause: io.EOF, Retryable: true},
	}
	resolver := NewLinkedInResolver(client, "")

	_, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err == nil {
		t.Fatal("expected error for network failure, got nil")
	}
}

func TestLinkedInResolver_Resolve_EnterpriseHash(t *testing.T) {
	client := &mockAuthenticatedClient{responseBody: "<html><body></body></html>", statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "enterprise-hash-123")

	_, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client.lastURL == "" {
		t.Fatal("expected client.Get to be called")
	}
	// The URL should contain the enterprise hash query parameter.
	if !contains(client.lastURL, "u=enterprise-hash-123") {
		t.Errorf("URL should contain enterprise hash parameter, got: %s", client.lastURL)
	}
}

func TestLinkedInResolver_Resolve_SkipsExerciseEntriesWithoutURL(t *testing.T) {
	files := []exerciseEntry{
		{Name: "has-url.zip", URL: "https://ambry.example.com/file.zip", SizeInBytes: 100},
		{Name: "no-url.zip", URL: "", SizeInBytes: 200},
	}
	htmlBody := buildTestHTML(exerciseBPR(files))

	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "my-course", "my-first-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files len = %d, want 1 (entry without URL skipped)", len(result.Files))
	}
	if result.Files[0].FileName != "has-url.zip" {
		t.Errorf("Files[0].FileName = %q, want %q", result.Files[0].FileName, "has-url.zip")
	}
}

func TestLinkedInResolver_InterfaceCompliance(_ *testing.T) {
	// Compile-time check is in linkedin.go (var _ Resolver = (*linkedinResolver)(nil)).
	_ = Resolver(&linkedinResolver{})
}

func TestBuildCoursePageURL(t *testing.T) {
	tests := []struct {
		name            string
		courseSlug      string
		firstVideoSlug  string
		enterpriseHash  string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "standard URL",
			courseSlug:      "learning-go",
			firstVideoSlug:  "welcome",
			enterpriseHash:  "",
			wantContains:    []string{"https://www.linkedin.com/learning/learning-go/welcome"},
			wantNotContains: []string{"u="},
		},
		{
			name:            "enterprise URL",
			courseSlug:      "learning-go",
			firstVideoSlug:  "welcome",
			enterpriseHash:  "ent-123",
			wantContains:    []string{"https://www.linkedin.com/learning/learning-go/welcome", "u=ent-123"},
			wantNotContains: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCoursePageURL(tt.courseSlug, tt.firstVideoSlug, tt.enterpriseHash)
			for _, substr := range tt.wantContains {
				if !contains(got, substr) {
					t.Errorf("URL missing %q, got: %s", substr, got)
				}
			}
			for _, substr := range tt.wantNotContains {
				if contains(got, substr) {
					t.Errorf("URL should not contain %q, got: %s", substr, got)
				}
			}
		})
	}
}

func TestExtractExerciseFilesFromHTML_MultipleBPRBlocks(t *testing.T) {
	// Page with multiple BPR blocks: one irrelevant type, one Course with files.
	bprCourse := exerciseBPR([]exerciseEntry{
		{Name: "ex.zip", URL: "https://ambry.example.com/ex.zip", SizeInBytes: 500},
	})
	bprOther := bprData{
		Included: []bprIncludedItem{
			{DollarType: "com.linkedin.some.other.Type"},
		},
	}
	otherJSON, _ := json.Marshal(bprOther)

	htmlBody := `<!DOCTYPE html><html><body>` +
		`<code id="bpr-guid-other">` + htmlEscape(string(otherJSON)) + `</code>` +
		`<code id="bpr-guid-course">` + htmlEscape(bprCourse) + `</code>` +
		`</body></html>`

	files, err := extractExerciseFilesFromHTML(htmlBody)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(files))
	}
	if files[0].FileName != "ex.zip" {
		t.Errorf("Files[0].FileName = %q, want %q", files[0].FileName, "ex.zip")
	}
}

func TestNewLinkedInResolver_WithHTTPTestServer(t *testing.T) {
	files := []exerciseEntry{
		{Name: "test.zip", URL: "https://ambry.example.com/test.zip", SizeInBytes: 999},
	}
	bprJSON := exerciseBPR(files)
	htmlBody := buildTestHTML(bprJSON)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(htmlBody))
	}))
	defer server.Close()

	// Verify the server serves valid HTML.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to test server: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("test server returned %d, want 200", resp.StatusCode)
	}

	// Use the mock client to verify Resolve with the server response body.
	client := &mockAuthenticatedClient{responseBody: htmlBody, statusCode: http.StatusOK}
	resolver := NewLinkedInResolver(client, "")

	result, err := resolver.Resolve(context.Background(), "test-course", "test-video")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("Files len = %d, want 1", len(result.Files))
	}
	if result.Files[0].FileName != "test.zip" {
		t.Errorf("Files[0].FileName = %q, want %q", result.Files[0].FileName, "test.zip")
	}
	if result.Files[0].FileSize != 999 {
		t.Errorf("Files[0].FileSize = %d, want 999", result.Files[0].FileSize)
	}
}

// contains is a simple string containment check to avoid importing strings in tests.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
