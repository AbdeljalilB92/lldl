package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AbdeljalilB92/lldl/features/auth"
	"github.com/AbdeljalilB92/lldl/features/config"
	"github.com/AbdeljalilB92/lldl/features/course"
	"github.com/AbdeljalilB92/lldl/features/exercise"
	"github.com/AbdeljalilB92/lldl/features/video"
	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
)

// --- Compile-time interface assertions for mocks ---

var _ auth.Provider = (*mockAuthProvider)(nil)

var _ config.Store = (*mockConfigStore)(nil)

var _ course.Fetcher = (*mockCourseFetcher)(nil)

var _ video.Resolver = (*mockVideoResolver)(nil)

var _ exercise.Resolver = (*mockExerciseResolver)(nil)

var _ sharedhttp.AuthenticatedClient = (*mockAuthenticatedClient)(nil)

// --- Mock implementations ---

// mockAuthProvider succeeds for any token with length >= 10.
type mockAuthProvider struct {
	err error
}

func (m *mockAuthProvider) Authenticate(_ context.Context, _ string) (auth.Result, error) {
	if m.err != nil {
		return auth.Result{}, m.err
	}
	// Return a minimal valid result with a mock client.
	return auth.Result{
		Client:         &mockAuthenticatedClient{},
		CSRFToken:      "test-csrf",
		EnterpriseHash: "",
	}, nil
}

// mockConfigStore stores config in memory.
type mockConfigStore struct {
	cfg *config.Config
}

func (m *mockConfigStore) Load() (*config.Config, error) {
	if m.cfg == nil {
		return nil, fmt.Errorf("no config")
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *config.Config) error {
	m.cfg = cfg
	return nil
}

// mockCourseFetcher returns a fixed course structure.
type mockCourseFetcher struct {
	course *course.Course
	err    error
}

func (m *mockCourseFetcher) FetchCourse(_ context.Context, _ string) (*course.Course, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.course, nil
}

// mockVideoResolver returns a fixed video result.
type mockVideoResolver struct {
	result *video.Result
	err    error
}

func (m *mockVideoResolver) Resolve(_ context.Context, _, _ string) (*video.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockExerciseResolver returns a fixed exercise result.
type mockExerciseResolver struct {
	result *exercise.Result
	err    error
}

func (m *mockExerciseResolver) Resolve(_ context.Context, _, _ string) (*exercise.Result, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// mockAuthenticatedClient satisfies sharedhttp.AuthenticatedClient.
// When testServerURL is set, Get proxies requests to that server for integration tests.
type mockAuthenticatedClient struct {
	testServerURL string
}

func (m *mockAuthenticatedClient) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	if m.testServerURL != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		return http.DefaultClient.Do(req)
	}
	return nil, fmt.Errorf("not implemented")
}

func (m *mockAuthenticatedClient) GetWithRetry(ctx context.Context, rawURL string, _ int) ([]byte, error) {
	resp, err := m.Get(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return data, err
}

// --- Helpers ---

func newTestService() *WailsService {
	return NewWailsService(&mockAuthProvider{}, &mockConfigStore{})
}

func newTestServiceWithConfig(cfg *config.Config) *WailsService {
	return NewWailsService(&mockAuthProvider{}, &mockConfigStore{cfg: cfg})
}

func newTestServiceWithAuthError(err error) *WailsService {
	return NewWailsService(&mockAuthProvider{err: err}, &mockConfigStore{})
}

// --- Tests ---

func TestWailsService_LoadConfig_Found(t *testing.T) {
	cfg := &config.Config{
		AuthToken:       "saved-token-12345",
		Quality:         "720",
		CourseDirectory: "/tmp/downloads",
	}
	svc := newTestServiceWithConfig(cfg)
	svc.ctx = context.Background()

	resp := svc.LoadConfig()
	if !resp.Found {
		t.Fatal("expected Found=true")
	}
	if resp.Token != "saved-token-12345" {
		t.Errorf("expected token %q, got %q", "saved-token-12345", resp.Token)
	}
	if resp.Quality != "720" {
		t.Errorf("expected quality %q, got %q", "720", resp.Quality)
	}
	if resp.OutputDir != "/tmp/downloads" {
		t.Errorf("expected outputDir %q, got %q", "/tmp/downloads", resp.OutputDir)
	}
}

func TestWailsService_LoadConfig_NotFound(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	resp := svc.LoadConfig()
	if resp.Found {
		t.Fatal("expected Found=false when no config exists")
	}
}

func TestWailsService_Authenticate_Success(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	resp := svc.Authenticate("valid-token-123456")
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}
	if svc.courseFetcher == nil {
		t.Error("expected courseFetcher to be initialized")
	}
	if svc.videoResolver == nil {
		t.Error("expected videoResolver to be initialized")
	}
	if svc.exerciseResolver == nil {
		t.Error("expected exerciseResolver to be initialized")
	}
	if svc.authClient == nil {
		t.Error("expected authClient to be initialized")
	}
}

func TestWailsService_Authenticate_EmptyToken(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	resp := svc.Authenticate("")
	if resp.Success {
		t.Fatal("expected failure for empty token")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestWailsService_Authenticate_ShortToken(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	resp := svc.Authenticate("short")
	if resp.Success {
		t.Fatal("expected failure for short token")
	}
}

func TestWailsService_Authenticate_AuthError(t *testing.T) {
	svc := newTestServiceWithAuthError(fmt.Errorf("network failure"))
	svc.ctx = context.Background()

	resp := svc.Authenticate("valid-token-123456")
	if resp.Success {
		t.Fatal("expected failure when auth provider returns error")
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestWailsService_FetchCourse_Success(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	// Authenticate first to initialize courseFetcher.
	svc.Authenticate("valid-token-123456")

	// Replace courseFetcher with a mock that returns a predictable course.
	svc.courseFetcher = &mockCourseFetcher{
		course: &course.Course{
			Title: "Test Course",
			Slug:  "test-course",
			Chapters: []course.Chapter{
				{
					Title:         "Chapter 1",
					Slug:          "ch1",
					IndexInCourse: 0,
					Videos: []course.Video{
						{Title: "Video 1", Slug: "v1", Duration: 120},
						{Title: "Video 2", Slug: "v2", Duration: 300},
					},
				},
			},
		},
	}

	resp := svc.FetchCourse("https://www.linkedin.com/learning/test-course")

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Title != "Test Course" {
		t.Errorf("expected title %q, got %q", "Test Course", resp.Title)
	}
	if resp.ChapterCount != 1 {
		t.Errorf("expected 1 chapter, got %d", resp.ChapterCount)
	}
	if resp.VideoCount != 2 {
		t.Errorf("expected 2 videos, got %d", resp.VideoCount)
	}
	if len(resp.Chapters) != 1 {
		t.Fatalf("expected 1 chapter in response, got %d", len(resp.Chapters))
	}
	if len(resp.Chapters[0].Videos) != 2 {
		t.Errorf("expected 2 videos in chapter, got %d", len(resp.Chapters[0].Videos))
	}
	if svc.course == nil {
		t.Error("expected course to be stored on service")
	}
}

func TestWailsService_FetchCourse_InvalidURL(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	resp := svc.FetchCourse("not-a-valid-url")
	if resp.Error == "" {
		t.Error("expected error for invalid URL")
	}
}

func TestWailsService_FetchCourse_NotAuthenticated(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	resp := svc.FetchCourse("https://www.linkedin.com/learning/test-course")
	if resp.Error == "" {
		t.Error("expected error when not authenticated")
	}
}

func TestWailsService_FetchCourse_FetchError(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	svc.courseFetcher = &mockCourseFetcher{
		err: fmt.Errorf("API failure"),
	}

	resp := svc.FetchCourse("https://www.linkedin.com/learning/test-course")
	if resp.Error == "" {
		t.Error("expected error when fetch fails")
	}
}

func TestWailsService_SaveConfig(t *testing.T) {
	store := &mockConfigStore{}
	svc := NewWailsService(&mockAuthProvider{}, store)
	svc.ctx = context.Background()

	err := svc.SaveConfig(SaveConfigRequest{
		Token:     "test-token",
		Quality:   "720",
		OutputDir: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.cfg.AuthToken != "test-token" {
		t.Errorf("expected token %q, got %q", "test-token", store.cfg.AuthToken)
	}
	if store.cfg.Quality != "720" {
		t.Errorf("expected quality %q, got %q", "720", store.cfg.Quality)
	}
}

func TestWailsService_ResolveVideos_NoCourse(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	err := svc.ResolveVideos()
	if err == nil {
		t.Fatal("expected error when no course loaded")
	}
}

func TestWailsService_ResolveVideos_NotAuthenticated(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.course = &course.Course{Title: "Test"}

	err := svc.ResolveVideos()
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestWailsService_ResolveVideos_Success(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	svc.course = &course.Course{
		Title: "Test",
		Slug:  "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "V1", Slug: "v1"},
				},
			},
		},
	}

	svc.videoResolver = &mockVideoResolver{
		result: &video.Result{
			DownloadURL: "https://cdn.example.com/v1.mp4",
			Duration:    120,
		},
	}

	err := svc.ResolveVideos()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.course.Chapters[0].Videos[0].DownloadURL != "https://cdn.example.com/v1.mp4" {
		t.Errorf("expected download URL to be set, got %q", svc.course.Chapters[0].Videos[0].DownloadURL)
	}
}

func TestWailsService_ResolveVideos_AllFail(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	svc.course = &course.Course{
		Title: "Test",
		Slug:  "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "V1", Slug: "v1"},
				},
			},
		},
	}

	svc.videoResolver = &mockVideoResolver{
		err: fmt.Errorf("resolution failed"),
	}

	err := svc.ResolveVideos()
	if err == nil {
		t.Fatal("expected error when all videos fail")
	}
}

func TestWailsService_ResolveExercises_NoCourse(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	err := svc.ResolveExercises()
	if err == nil {
		t.Fatal("expected error when no course loaded")
	}
}

func TestWailsService_ResolveExercises_Success(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	svc.course = &course.Course{
		Title: "Test",
		Slug:  "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos:        []course.Video{{Title: "V1", Slug: "v1"}},
			},
		},
		ExerciseFiles: []shareddomain.ExerciseFile{
			{FileName: "old.zip", DownloadURL: "https://old.example.com/old.zip"},
		},
	}

	svc.exerciseResolver = &mockExerciseResolver{
		result: &exercise.Result{
			Files: []shareddomain.ExerciseFile{
				{FileName: "exercises.zip", DownloadURL: "https://cdn.example.com/exercises.zip"},
			},
		},
	}

	err := svc.ResolveExercises()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(svc.course.ExerciseFiles) != 1 {
		t.Fatalf("expected 1 exercise file, got %d", len(svc.course.ExerciseFiles))
	}
	if svc.course.ExerciseFiles[0].FileName != "exercises.zip" {
		t.Errorf("expected exercises.zip, got %q", svc.course.ExerciseFiles[0].FileName)
	}
}

func TestWailsService_ResolveExercises_NoExerciseFiles(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")

	svc.course = &course.Course{
		Title:         "Test",
		Slug:          "test-course",
		Chapters:      []course.Chapter{},
		ExerciseFiles: nil,
	}

	// Should succeed without calling resolver when there are no exercise files.
	err := svc.ResolveExercises()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWailsService_StartDownload_NoCourse(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()

	err := svc.StartDownload()
	if err == nil {
		t.Fatal("expected error when no course loaded")
	}
}

func TestWailsService_StartDownload_NotAuthenticated(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.course = &course.Course{Title: "Test"}

	err := svc.StartDownload()
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestWailsService_StartDownload_NoOutputDir(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")
	svc.course = &course.Course{Title: "Test"}

	err := svc.StartDownload()
	if err == nil {
		t.Fatal("expected error when output dir not set")
	}
}

func TestWailsService_Cancel_NilCancelFunc(t *testing.T) {
	svc := newTestService()
	// Should not panic when cancelFunc is nil.
	err := svc.Cancel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWailsService_SetQuality(t *testing.T) {
	svc := newTestService()

	svc.SetQuality("high")
	if svc.quality != shareddomain.QualityHigh {
		t.Errorf("expected QualityHigh, got %v", svc.quality)
	}

	svc.SetQuality("medium")
	if svc.quality != shareddomain.QualityMedium {
		t.Errorf("expected QualityMedium, got %v", svc.quality)
	}

	// Invalid quality should default to high.
	svc.SetQuality("invalid")
	if svc.quality != shareddomain.QualityHigh {
		t.Errorf("expected QualityHigh fallback, got %v", svc.quality)
	}
}

func TestWailsService_SetOutputDir(t *testing.T) {
	svc := newTestService()
	if err := svc.SetOutputDir("/tmp/downloads"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.outputDir != "/tmp/downloads" {
		t.Errorf("expected /tmp/downloads, got %q", svc.outputDir)
	}
}

func TestWailsService_SetOutputDir_RejectsEmpty(t *testing.T) {
	svc := newTestService()
	if err := svc.SetOutputDir(""); err == nil {
		t.Fatal("expected error for empty dir")
	}
}

func TestWailsService_SetOutputDir_RejectsRelative(t *testing.T) {
	svc := newTestService()
	if err := svc.SetOutputDir("relative/path"); err == nil {
		t.Fatal("expected error for relative path")
	}
}

func TestWailsService_SetOutputDir_RejectsTraversal(t *testing.T) {
	svc := newTestService()
	if err := svc.SetOutputDir("/tmp/../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestWailsService_OnStartup(t *testing.T) {
	svc := newTestService()
	ctx := context.Background()
	svc.OnStartup(ctx)
	if svc.ctx == nil {
		t.Error("expected ctx to be set")
	}
}

func TestWailsService_StartDownload_Success(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")
	if err := svc.SetOutputDir(t.TempDir()); err != nil {
		t.Fatalf("SetOutputDir failed: %v", err)
	}

	svc.course = &course.Course{
		Title: "Test Course",
		Slug:  "test-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "V1", Slug: "v1", DownloadURL: "http://example.com/v1.mp4"},
				},
			},
		},
	}

	// Use an HTTP test server so the real concurrent engine can download successfully.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	}))
	defer ts.Close()

	// Override the video URL to point at the test server.
	svc.course.Chapters[0].Videos[0].DownloadURL = ts.URL + "/v1.mp4"
	svc.authClient = &mockAuthenticatedClient{testServerURL: ts.URL}

	err := svc.StartDownload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWailsService_StartDownload_WithFailures(t *testing.T) {
	svc := newTestService()
	svc.ctx = context.Background()
	svc.Authenticate("valid-token-123456")
	if err := svc.SetOutputDir(t.TempDir()); err != nil {
		t.Fatalf("SetOutputDir failed: %v", err)
	}

	svc.course = &course.Course{
		Title: "Fail Course",
		Slug:  "fail-course",
		Chapters: []course.Chapter{
			{
				Title:         "Ch1",
				IndexInCourse: 0,
				Videos: []course.Video{
					{Title: "V1", Slug: "v1", DownloadURL: "http://example.com/v1.mp4"},
				},
			},
		},
	}

	// Use an HTTP test server that returns 500 so the download fails critically.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	svc.course.Chapters[0].Videos[0].DownloadURL = ts.URL + "/v1.mp4"
	svc.authClient = &mockAuthenticatedClient{testServerURL: ts.URL}

	err := svc.StartDownload()
	if err == nil {
		t.Fatal("expected error when critical downloads fail")
	}
}

func TestWireForGUI(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	svc := WireForGUI(WireGUIConfig{
		Concurrency: 2,
		Delay:       0,
		ConfigPath:  cfgPath,
	})

	if svc == nil {
		t.Fatal("expected non-nil WailsService")
	}
	if svc.authProvider == nil {
		t.Error("expected authProvider to be set")
	}
	if svc.configStore == nil {
		t.Error("expected configStore to be set")
	}
	if svc.concurrency != 2 {
		t.Errorf("expected concurrency 2, got %d", svc.concurrency)
	}
}

func TestWireForGUI_DefaultConfigPath(t *testing.T) {
	svc := WireForGUI(WireGUIConfig{})
	if svc == nil {
		t.Fatal("expected non-nil WailsService")
	}
	// Should not panic when ConfigPath is empty — defaults to OS config dir.
	if svc.concurrency != 4 {
		t.Errorf("expected default concurrency 4, got %d", svc.concurrency)
	}
}

func TestWireForGUI_SavesAndLoads(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	svc := WireForGUI(WireGUIConfig{ConfigPath: cfgPath})
	svc.ctx = context.Background()

	// Save a config.
	err := svc.SaveConfig(SaveConfigRequest{
		Token:     "persisted-token-12345",
		Quality:   "540",
		OutputDir: "/tmp/persisted",
	})
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify the file was created.
	if _, statErr := os.Stat(cfgPath); statErr != nil {
		t.Fatalf("config file not created: %v", statErr)
	}

	// Load it back via a new service using the same path.
	svc2 := WireForGUI(WireGUIConfig{ConfigPath: cfgPath})
	svc2.ctx = context.Background()
	resp := svc2.LoadConfig()

	if !resp.Found {
		t.Fatal("expected config to be found")
	}
	if resp.Quality != "540" {
		t.Errorf("expected quality 540, got %q", resp.Quality)
	}
}
