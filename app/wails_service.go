package app

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/AbdeljalilB92/lldl/features/auth"
	"github.com/AbdeljalilB92/lldl/features/config"
	"github.com/AbdeljalilB92/lldl/features/course"
	"github.com/AbdeljalilB92/lldl/features/download"
	"github.com/AbdeljalilB92/lldl/features/exercise"
	"github.com/AbdeljalilB92/lldl/features/video"
	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
	"github.com/AbdeljalilB92/lldl/shared/validation"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// WailsService exposes individual pipeline steps as Wails binding methods.
// The frontend calls each method in sequence instead of running a blocking
// CLI pipeline. Progress is communicated via Wails events.
type WailsService struct {
	ctx context.Context

	// Injected at construction — never nil.
	authProvider auth.Provider
	configStore  config.Store

	// Built during Authenticate — stored for subsequent steps.
	courseFetcher    course.Fetcher
	videoResolver    video.Resolver
	exerciseResolver exercise.Resolver

	// HTTP client retained from Authenticate so the download engine can be
	// recreated with a progress callback in StartDownload.
	authClient sharedhttp.AuthenticatedClient

	// Accumulated state across method calls.
	token     string
	quality   shareddomain.Quality
	outputDir string
	course    *course.Course

	// Allows Cancel() to abort in-progress resolve/download.
	cancelFunc context.CancelFunc

	// Protects shared mutable state (quality, outputDir, cancelFunc, token, course).
	mu sync.Mutex

	// Tuneable settings.
	concurrency int
	delay       int
}

// emitEvent safely emits a Wails event. When the context is not a valid
// Wails context (e.g. context.Background() in unit tests), the call is
// silently skipped because the Wails runtime would call log.Fatalf otherwise.
func (s *WailsService) emitEvent(name string, data ...interface{}) {
	if s.ctx == nil {
		return
	}
	// The Wails runtime stores a frontend.Events instance under the "events"
	// context key. If absent, EventsEmit calls log.Fatalf — guard against that.
	if s.ctx.Value("events") == nil {
		return
	}
	runtime.EventsEmit(s.ctx, name, data...)
}

// NewWailsService creates a WailsService with injected auth and config providers.
// Authenticated features (course/video/exercise/download) are built later when
// Authenticate is called, because they depend on the validated token.
func NewWailsService(authProvider auth.Provider, configStore config.Store) *WailsService {
	return &WailsService{
		authProvider: authProvider,
		configStore:  configStore,
		quality:      shareddomain.QualityHigh,
		concurrency:  4,
		delay:        1,
	}
}

// OnStartup stores the Wails context. Called automatically by the Wails runtime.
func (s *WailsService) OnStartup(ctx context.Context) {
	s.ctx = ctx
}

// OnShutdown cleans up resources. Called automatically by the Wails runtime.
func (s *WailsService) OnShutdown(_ context.Context) {
	s.mu.Lock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	s.mu.Unlock()
}

// LoadConfig returns the saved configuration so the frontend can pre-fill forms.
// Returns ConfigResponse.Found=false when no config exists.
func (s *WailsService) LoadConfig() *ConfigResponse {
	cfg, err := s.configStore.Load()
	if err != nil || cfg == nil {
		return &ConfigResponse{Found: false}
	}
	return &ConfigResponse{
		Token:     cfg.AuthToken,
		Quality:   cfg.Quality,
		OutputDir: cfg.CourseDirectory,
		CourseURL: cfg.CourseURL,
		Found:     true,
	}
}

// SaveConfig persists user settings for the next session.
func (s *WailsService) SaveConfig(req SaveConfigRequest) error {
	cfg := &config.Config{
		AuthToken:       req.Token,
		Quality:         req.Quality,
		CourseDirectory: req.OutputDir,
		CourseURL:       req.CourseURL,
	}
	return s.configStore.Save(cfg)
}

// Authenticate validates the token and builds authenticated feature implementations.
// Must be called before FetchCourse, ResolveVideos, etc.
func (s *WailsService) Authenticate(token string) *AuthResponse {
	logger := logging.New("[GUI][Authenticate]")

	if err := validation.ValidateToken(token); err != nil {
		return &AuthResponse{Success: false, Error: err.Error()}
	}

	authResult, err := s.authProvider.Authenticate(s.ctx, token)
	if err != nil {
		logger.Warn("authentication failed", "error", err)
		return &AuthResponse{Success: false, Error: err.Error()}
	}

	// Store token and quality for later steps.
	s.mu.Lock()
	s.token = token
	quality := s.quality
	s.mu.Unlock()

	// Build authenticated features — same as CLI step 4.
	client := authResult.Client
	s.authClient = client
	s.courseFetcher = course.NewLinkedInFetcher(client)
	s.videoResolver = video.NewLinkedInResolver(client, quality)
	s.exerciseResolver = exercise.NewLinkedInResolver(client, authResult.EnterpriseHash)

	return &AuthResponse{Success: true}
}

// FetchCourse validates the course URL, fetches the course structure, and stores it.
// Must be called after Authenticate.
func (s *WailsService) FetchCourse(courseURL string) *CourseResponse {
	logger := logging.New("[GUI][FetchCourse]")

	if s.courseFetcher == nil {
		return &CourseResponse{Error: "not authenticated — call Authenticate first"}
	}

	courseSlug, err := validation.ValidateCourseURL(courseURL)
	if err != nil {
		return &CourseResponse{Error: fmt.Sprintf("invalid course URL: %v", err)}
	}

	crs, err := s.courseFetcher.FetchCourse(s.ctx, courseSlug)
	if err != nil {
		logger.Warn("course fetch failed", "slug", courseSlug, "error", err)
		return &CourseResponse{Error: fmt.Sprintf("failed to fetch course: %v", err)}
	}

	// The LinkedIn API response does not include a slug field, so preserve the
	// one extracted from the user-supplied URL for subsequent resolve calls.
	crs.Slug = courseSlug

	s.mu.Lock()
	s.course = crs
	s.mu.Unlock()

	// Count total videos for the response.
	totalVideos := 0
	for _, ch := range crs.Chapters {
		totalVideos += len(ch.Videos)
	}

	resp := &CourseResponse{
		Title:        crs.Title,
		ChapterCount: len(crs.Chapters),
		VideoCount:   totalVideos,
		HasExercises: len(crs.ExerciseFiles) > 0,
		Chapters:     make([]ChapterResponse, len(crs.Chapters)),
	}

	for i, ch := range crs.Chapters {
		resp.Chapters[i] = ChapterResponse{
			Title:  ch.Title,
			Slug:   ch.Slug,
			Videos: make([]VideoResponse, len(ch.Videos)),
		}
		for j, vid := range ch.Videos {
			resp.Chapters[i].Videos[j] = VideoResponse{
				Title:    vid.Title,
				Slug:     vid.Slug,
				Duration: vid.Duration,
			}
		}
	}

	return resp
}

// ResolveVideos resolves download URLs and transcripts for all videos in the
// stored course. Emits "resolve:progress" events after each video so the
// frontend can update a progress bar. Must be called after FetchCourse.
func (s *WailsService) ResolveVideos() error {
	logger := logging.New("[GUI][ResolveVideos]")

	s.mu.Lock()
	courseVal := s.course
	videoResolver := s.videoResolver
	s.mu.Unlock()

	if courseVal == nil {
		return fmt.Errorf("no course loaded — call FetchCourse first")
	}
	if videoResolver == nil {
		return fmt.Errorf("not authenticated — call Authenticate first")
	}

	// Create a cancellable context for the resolve loop.
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelFunc = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	crs := courseVal
	totalVideos := 0
	for ci := range crs.Chapters {
		totalVideos += len(crs.Chapters[ci].Videos)
	}

	resolveCount := 0
	resolveFailures := 0

	for ci := range crs.Chapters {
		for vi := range crs.Chapters[ci].Videos {
			vid := &crs.Chapters[ci].Videos[vi]
			resolveCount++

			progress := &ResolveProgress{
				Current: resolveCount,
				Total:   totalVideos,
				Title:   vid.Title,
			}
			s.emitEvent("resolve:progress", progress)

			vResult, err := videoResolver.Resolve(ctx, crs.Slug, vid.Slug)
			if err != nil {
				resolveFailures++
				logger.Warn("video resolution failed", "slug", vid.Slug, "error", err)

				failProgress := &ResolveProgress{
					Current: resolveCount,
					Total:   totalVideos,
					Title:   vid.Title,
					Error:   err.Error(),
				}
				s.emitEvent("resolve:progress", failProgress)
				continue
			}
			vid.DownloadURL = vResult.DownloadURL
			vid.TranscriptLines = vResult.TranscriptLines
			vid.FormTranscript()
			if vResult.Duration > 0 {
				vid.Duration = vResult.Duration
			}

			// Throttle between requests to avoid rate-limiting on large courses.
			if s.delay > 0 && resolveCount < totalVideos {
				select {
				case <-time.After(time.Duration(s.delay) * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	if resolveFailures == totalVideos && totalVideos > 0 {
		return fmt.Errorf("all %d video(s) failed to resolve", totalVideos)
	}

	return nil
}

// ResolveExercises resolves exercise file URLs for the stored course.
// Must be called after FetchCourse and Authenticate.
func (s *WailsService) ResolveExercises() error {
	s.mu.Lock()
	courseVal := s.course
	exerciseResolver := s.exerciseResolver
	s.mu.Unlock()

	if courseVal == nil {
		return fmt.Errorf("no course loaded — call FetchCourse first")
	}
	if exerciseResolver == nil {
		return fmt.Errorf("not authenticated — call Authenticate first")
	}

	crs := courseVal
	if len(crs.ExerciseFiles) == 0 || len(crs.Chapters) == 0 || len(crs.Chapters[0].Videos) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelFunc = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	firstVideoSlug := crs.Chapters[0].Videos[0].Slug
	exResult, err := exerciseResolver.Resolve(ctx, crs.Slug, firstVideoSlug)
	if err != nil {
		return fmt.Errorf("could not fetch exercise file URLs: %w", err)
	}
	if len(exResult.Files) > 0 {
		crs.ExerciseFiles = exResult.Files
	}

	return nil
}

// StartDownload builds download jobs from the stored course and executes them.
// Emits "download:progress" events per job via a progress callback and
// "download:complete" at the end. Must be called after ResolveVideos.
func (s *WailsService) StartDownload() error {
	logger := logging.New("[GUI][StartDownload]")

	s.mu.Lock()
	courseVal := s.course
	authClientVal := s.authClient
	outputDirVal := s.outputDir
	s.mu.Unlock()

	if courseVal == nil {
		return fmt.Errorf("no course loaded — call FetchCourse first")
	}
	if authClientVal == nil {
		return fmt.Errorf("not authenticated — call Authenticate first")
	}
	if outputDirVal == "" {
		return fmt.Errorf("output directory not set — call SaveConfig with outputDir first")
	}

	// Save config before download so credentials persist on crash.
	s.saveCurrentConfig(logger)

	// Create a cancellable context for downloads.
	ctx, cancel := context.WithCancel(s.ctx)
	s.mu.Lock()
	s.cancelFunc = cancel
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.cancelFunc = nil
		s.mu.Unlock()
	}()

	jobs := buildDownloadJobs(courseVal, outputDirVal)

	s.emitEvent("download:start", &DownloadStartResponse{
		TotalJobs: len(jobs),
	})

	var succeeded, failed, skipped int
	var jobCounter int64

	// Recreate the engine with a progress callback so events fire in real-time
	// as each job finishes, rather than after all downloads complete.
	engine := download.NewConcurrentEngine(s.concurrency, authClientVal,
		download.WithProgress(func(_, _ int, description string, err error, skipped bool) {
			n := atomic.AddInt64(&jobCounter, 1)
			status := "succeeded"
			if skipped {
				status = "skipped"
			} else if err != nil {
				status = "failed"
			}
			s.emitEvent("download:progress", &DownloadProgressEvent{
				JobID:    fmt.Sprintf("job-%d", n),
				FileName: description,
				Status:   status,
				Error: func() string {
					if err != nil {
						return err.Error()
					}
					return ""
				}(),
			})
		}),
	)

	results := engine.DownloadAll(ctx, jobs)

	// Tally final results for the completion event. Non-critical DNS failures
	// are marked Skipped by the engine and emitted as "failed" during the real-time
	// progress callback (since skip status isn't known until after the callback fires).
	// The completion event provides the accurate final counts.
	for _, r := range results {
		if r.Err != nil && r.Critical {
			failed++
			logger.Error("download failed", "file", r.Description, "error", r.Err)
		} else if r.Skipped {
			skipped++
		} else if r.Err != nil {
			logger.Warn("non-critical download issue", "file", r.Description, "error", r.Err)
		} else {
			succeeded++
		}
	}

	completeEvt := &DownloadCompleteResponse{
		Succeeded: succeeded,
		Failed:    failed,
		Skipped:   skipped,
	}
	if failed > 0 {
		completeEvt.Error = fmt.Sprintf("%d critical download(s) failed", failed)
	}

	s.emitEvent("download:complete", completeEvt)

	if failed > 0 {
		return fmt.Errorf("%d critical download(s) failed", failed)
	}

	return nil
}

// Cancel aborts any in-progress resolve or download operation.
func (s *WailsService) Cancel() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFunc != nil {
		s.cancelFunc()
		s.cancelFunc = nil
	}
	return nil
}

// SetQuality stores the selected quality. Must be called before Authenticate
// because the video resolver needs quality at construction time.
func (s *WailsService) SetQuality(quality string) {
	q, err := shareddomain.QualityFromString(quality)
	if err != nil {
		q = shareddomain.QualityHigh
	}
	s.mu.Lock()
	s.quality = q
	s.mu.Unlock()
}

// SetOutputDir stores the download output directory. Rejects empty paths,
// relative paths, and paths containing ".." to prevent directory traversal.
func (s *WailsService) SetOutputDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("output directory must not be empty")
	}

	if strings.Contains(dir, "..") {
		return fmt.Errorf("output directory must not contain path traversal (..): %s", dir)
	}

	cleaned := filepath.Clean(dir)

	if !filepath.IsAbs(cleaned) {
		return fmt.Errorf("output directory must be an absolute path: %s", dir)
	}

	s.mu.Lock()
	s.outputDir = cleaned
	s.mu.Unlock()
	return nil
}

// saveCurrentConfig persists the current token, quality, and output directory.
func (s *WailsService) saveCurrentConfig(logger *slog.Logger) {
	s.mu.Lock()
	token := s.token
	quality := s.quality
	outputDir := s.outputDir
	s.mu.Unlock()

	cfg := &config.Config{
		AuthToken:       token,
		Quality:         quality.String(),
		CourseDirectory: outputDir,
	}
	if err := s.configStore.Save(cfg); err != nil {
		logger.Warn("failed to save config", "error", err)
	}
}
