// Package app orchestrates the full LinkedIn Learning course download flow.
// It is the ONLY package that imports concrete types from multiple feature
// modules. Business logic lives here; wiring lives in wire.go.
package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/AbdeljalilB92/lldl/features/auth"
	"github.com/AbdeljalilB92/lldl/features/config"
	"github.com/AbdeljalilB92/lldl/features/course"
	"github.com/AbdeljalilB92/lldl/features/download"
	"github.com/AbdeljalilB92/lldl/features/exercise"
	"github.com/AbdeljalilB92/lldl/features/ui"
	"github.com/AbdeljalilB92/lldl/features/video"
	"github.com/AbdeljalilB92/lldl/lib/sanitize"
	shareddomain "github.com/AbdeljalilB92/lldl/shared/domain"
	"github.com/AbdeljalilB92/lldl/shared/logging"
	"github.com/AbdeljalilB92/lldl/shared/validation"
)

// App coordinates the full download pipeline: authenticate, fetch course,
// resolve video URLs, build download jobs, and execute them.
type App struct {
	authProvider     auth.Provider
	courseFetcher    course.Fetcher
	videoResolver    video.Resolver
	exerciseResolver exercise.Resolver
	downloadEngine   download.Engine
	configStore      config.Store
	presenter        ui.Presenter
	concurrency      int
	delay            int
}

// Option applies a configuration tweak to App during construction.
type Option func(*App)

// WithConcurrency sets the worker pool size for downloads.
func WithConcurrency(n int) Option {
	return func(a *App) { a.concurrency = n }
}

// WithDelay sets the inter-request delay (in seconds) for course fetching.
func WithDelay(d int) Option {
	return func(a *App) { a.delay = d }
}

// New creates an App with the given feature implementations and optional
// configuration overrides.
func New(
	authProvider auth.Provider,
	courseFetcher course.Fetcher,
	videoResolver video.Resolver,
	exerciseResolver exercise.Resolver,
	downloadEngine download.Engine,
	configStore config.Store,
	presenter ui.Presenter,
	opts ...Option,
) *App {
	a := &App{
		authProvider:     authProvider,
		courseFetcher:    courseFetcher,
		videoResolver:    videoResolver,
		exerciseResolver: exerciseResolver,
		downloadEngine:   downloadEngine,
		configStore:      configStore,
		presenter:        presenter,
		concurrency:      4,
		delay:            1,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run executes the full download pipeline: config loading, authentication,
// course fetching, video/exercise resolution, and download execution.
func (a *App) Run(ctx context.Context) error {
	var (
		token     string
		quality   shareddomain.Quality
		outputDir string
	)

	// Step 1: Load existing config or prompt for values.
	savedCfg, err := a.configStore.Load()
	if err == nil && savedCfg != nil {
		a.presenter.ShowInfo("Found existing config file")
		a.presenter.ShowInfo(fmt.Sprintf("  Saved quality: %s, Saved directory: %s",
			savedCfg.Quality, savedCfg.CourseDirectory))
		if a.presenter.PromptYesNo("Would you like to reuse the saved configuration?") {
			token = savedCfg.AuthToken
			q, qErr := shareddomain.QualityFromString(savedCfg.Quality)
			if qErr != nil {
				a.presenter.ShowInfo(fmt.Sprintf("Invalid saved quality %q, defaulting to 720p", savedCfg.Quality))
				q = shareddomain.QualityHigh
			}
			quality = q
			outputDir = savedCfg.CourseDirectory
			a.presenter.ShowSuccess("Configuration loaded")
		} else {
			savedCfg = nil
		}
	}

	if savedCfg == nil {
		token = a.presenter.PromptPassword("Enter your li_at token:")
		q, err := a.presenter.PromptQuality()
		if err != nil {
			return fmt.Errorf("quality prompt failed: %w", err)
		}
		quality = q
		outputDir, err = a.presenter.PromptPath("Enter download directory path:")
		if err != nil {
			return fmt.Errorf("path prompt failed: %w", err)
		}

		savedCfg = &config.Config{
			AuthToken:       token,
			CourseDirectory: outputDir,
			Quality:         quality.String(),
		}
	}

	// Step 2: Prompt for course URL and extract slug.
	courseURL, err := a.presenter.PromptString("Enter the LinkedIn Learning course URL:")
	if err != nil {
		return fmt.Errorf("course URL prompt failed: %w", err)
	}
	courseSlug, err := validation.ValidateCourseURL(courseURL)
	if err != nil {
		return fmt.Errorf("invalid course URL: %w", err)
	}

	// Step 3: Authenticate.
	a.presenter.ShowInfo("Validating token...")
	authResult, err := a.authProvider.Authenticate(ctx, token)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	a.presenter.ShowSuccess("Token is valid")

	// Step 4: Build authenticated feature implementations.
	client := authResult.Client
	a.courseFetcher = course.NewLinkedInFetcher(client)
	a.videoResolver = video.NewLinkedInResolver(client, quality)
	a.exerciseResolver = exercise.NewLinkedInResolver(client, authResult.EnterpriseHash)
	a.downloadEngine = download.NewConcurrentEngine(a.concurrency, client)

	// Step 5: Fetch course structure.
	a.presenter.ShowInfo("Extracting course data. This might take some time...")
	crs, err := a.courseFetcher.FetchCourse(ctx, courseSlug)
	if err != nil {
		return fmt.Errorf("failed to fetch course: %w", err)
	}
	a.presenter.ShowSuccess("Course extracted successfully")

	if len(crs.Chapters) == 0 {
		a.presenter.ShowInfo("Warning: course has no chapters")
	}

	// Step 6: Resolve video URLs and transcripts for every video.
	// Throttle requests to avoid rate-limiting. Skip individual failures
	// and only abort if all videos fail.
	logger := logging.New("[App][ResolveVideos]")
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

			a.presenter.ShowInfo(fmt.Sprintf("Resolving video %d/%d: %s", resolveCount, totalVideos, vid.Title))

			vResult, err := a.videoResolver.Resolve(ctx, courseSlug, vid.Slug)
			if err != nil {
				resolveFailures++
				logger.Warn("video resolution failed, skipping", "slug", vid.Slug, "error", err)
				a.presenter.ShowInfo(fmt.Sprintf("Warning: skipping video %q — %v", vid.Slug, err))
				continue
			}
			vid.DownloadURL = vResult.DownloadURL
			vid.TranscriptLines = vResult.TranscriptLines
			vid.FormTranscript()
			if vResult.Duration > 0 {
				vid.Duration = vResult.Duration
			}

			// Throttle between requests to avoid rate-limiting on large courses.
			if a.delay > 0 && resolveCount < totalVideos {
				select {
				case <-time.After(time.Duration(a.delay) * time.Second):
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}

	if resolveFailures == totalVideos && totalVideos > 0 {
		return fmt.Errorf("all %d video(s) failed to resolve", totalVideos)
	} else if resolveFailures > 0 {
		a.presenter.ShowInfo(fmt.Sprintf("Warning: %d of %d video(s) failed to resolve", resolveFailures, totalVideos))
	}

	// Step 7: Replace dead API exercise file URLs with working ambry URLs.
	if len(crs.ExerciseFiles) > 0 && len(crs.Chapters) > 0 && len(crs.Chapters[0].Videos) > 0 {
		firstVideoSlug := crs.Chapters[0].Videos[0].Slug
		exResult, err := a.exerciseResolver.Resolve(ctx, courseSlug, firstVideoSlug)
		if err != nil {
			a.presenter.ShowInfo(fmt.Sprintf("Warning: could not fetch exercise file URLs: %v", err))
		} else if len(exResult.Files) > 0 {
			crs.ExerciseFiles = exResult.Files
		}
	}

	// Step 8: Show course info.
	a.presenter.ShowCourseInfo(crs.Title, len(crs.Chapters), totalVideos)

	// Step 9: Save config before download so credentials persist on crash.
	savedCfg.CourseDirectory = outputDir
	savedCfg.Quality = quality.String()
	savedCfg.AuthToken = token
	if err := a.configStore.Save(savedCfg); err != nil {
		a.presenter.ShowError(fmt.Sprintf("Failed to save config: %v", err))
	} else {
		a.presenter.ShowSuccess("Configuration saved for next run")
	}

	// Step 10: Build download jobs and execute.
	a.presenter.ShowInfo("Starting download...")
	jobs := buildDownloadJobs(crs, outputDir)
	results := a.downloadEngine.DownloadAll(ctx, jobs)

	failures := 0
	for _, r := range results {
		if r.Err != nil && r.Critical {
			failures++
			a.presenter.ShowError(fmt.Sprintf("%s: %v", r.Description, r.Err))
		} else if r.Skipped {
			a.presenter.ShowInfo(fmt.Sprintf("Skipped: %s", r.Description))
		}
	}

	if failures > 0 {
		return fmt.Errorf("%d critical download(s) failed", failures)
	}

	a.presenter.ShowSuccess("Download complete!")
	return nil
}

// buildDownloadJobs translates a Course into download.Job entries for the
// engine. Videos become critical jobs; exercise files are non-critical.
func buildDownloadJobs(crs *course.Course, outputDir string) []download.Job {
	courseDir := filepath.Join(outputDir, sanitize.ToSafeFileName(crs.Title))

	// Pre-estimate capacity: one video job + one transcript per video,
	// plus one per exercise file. Transcript jobs are conditional so this
	// is a soft upper bound.
	videoCount := 0
	for _, ch := range crs.Chapters {
		videoCount += len(ch.Videos)
	}
	jobs := make([]download.Job, 0, videoCount*2+len(crs.ExerciseFiles))

	for _, ch := range crs.Chapters {
		chDir := filepath.Join(courseDir, download.FormatChapterDir(ch.Title, ch.IndexInCourse))

		for vi, vid := range ch.Videos {
			// Skip videos whose URL could not be resolved — attempting to
			// download with an empty URL would fail at the HTTP layer anyway.
			if vid.DownloadURL == "" {
				continue
			}

			videoFile := filepath.Join(chDir, download.FormatVideoFileWithIndex(vid.Title, vi, "mp4"))
			jobs = append(jobs, download.Job{
				URL:         vid.DownloadURL,
				DestPath:    videoFile,
				Description: fmt.Sprintf("%s/%s", ch.Title, vid.Title),
				Critical:    true,
			})

			// Write SRT transcript as a content job (no URL fetch needed).
			if vid.Transcript != "" {
				srtFile := filepath.Join(chDir, download.FormatVideoFileWithIndex(vid.Title, vi, "srt"))
				jobs = append(jobs, download.Job{
					DestPath:    srtFile,
					Description: fmt.Sprintf("%s/%s (transcript)", ch.Title, vid.Title),
					Critical:    false,
					Content:     []byte(vid.Transcript),
				})
			}
		}
	}

	for _, ef := range crs.ExerciseFiles {
		safeName := sanitize.ToSafeFileName(ef.FileName)
		jobs = append(jobs, download.Job{
			URL:         ef.DownloadURL,
			DestPath:    filepath.Join(courseDir, "Exercise Files", safeName),
			Description: fmt.Sprintf("Exercise File: %s", safeName),
			Critical:    false,
		})
	}

	return jobs
}
