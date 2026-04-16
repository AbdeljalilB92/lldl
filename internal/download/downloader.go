package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/abdeljalil/linkedin-learning-downloader/internal/model"
	"github.com/schollz/progressbar/v3"
)

type Downloader struct {
	outputDir   string
	concurrency int
	progressMu  sync.Mutex
	authClient  *http.Client // authenticated client for exercise files (ambry CDN)
	csrfToken   string
}

func NewDownloader(outputDir string, concurrency int) *Downloader {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &Downloader{outputDir: outputDir, concurrency: concurrency}
}

// SetAuthClient sets the authenticated HTTP client used for downloading
// exercise files that require LinkedIn session authentication.
func (d *Downloader) SetAuthClient(client *http.Client, csrfToken string) {
	d.authClient = client
	d.csrfToken = csrfToken
}

type downloadJob struct {
	url         string
	destPath    string
	description string
	critical    bool   // true for videos, false for exercise files
	content     string // if non-empty, write this string directly (for SRT)
}

type downloadResult struct {
	description string
	err         error
	critical    bool
	skipped     bool
}

func (d *Downloader) DownloadCourse(ctx context.Context, course *model.Course) error {
	courseDir := filepath.Join(d.outputDir, model.ToSafeFileName(course.Title))
	if err := os.MkdirAll(courseDir, 0755); err != nil {
		return fmt.Errorf("creating course directory: %w", err)
	}

	var jobs []downloadJob

	// Build download jobs for all chapters and videos
	for _, chapter := range course.Chapters {
		chapterDir := filepath.Join(courseDir, model.FormatChapterDir(chapter))
		if err := os.MkdirAll(chapterDir, 0755); err != nil {
			return fmt.Errorf("creating chapter directory: %w", err)
		}

		for j, video := range chapter.Videos {
			videoFile := model.FormatVideoFileWithIndex(video, j, ".mp4")
			destPath := filepath.Join(chapterDir, videoFile)

			// Video download job
			jobs = append(jobs, downloadJob{
				url:         video.DownloadURL,
				destPath:    destPath,
				description: fmt.Sprintf("Video: %s", video.Title),
				critical:    true,
			})

			// SRT subtitle job
			if video.Transcript != "" {
				srtFile := model.FormatVideoFileWithIndex(video, j, ".srt")
				srtPath := filepath.Join(chapterDir, srtFile)
				jobs = append(jobs, downloadJob{
					url:         "",
					destPath:    srtPath,
					description: fmt.Sprintf("Subtitle: %s", video.Title),
					critical:    true,
					content:     video.Transcript,
				})
			}
		}
	}

	// Exercise file jobs (non-critical, best-effort)
	for _, ef := range course.ExerciseFiles {
		fileName := model.ToSafeFileName(ef.FileName)
		if fileName == "" {
			fileName = "exercise_file"
		}
		jobs = append(jobs, downloadJob{
			url:         ef.DownloadURL,
			destPath:    filepath.Join(courseDir, fileName),
			description: fmt.Sprintf("Exercise: %s", ef.FileName),
			critical:    false,
		})
	}

	// Execute all jobs with worker pool
	results := d.downloadAll(ctx, jobs)

	// Summarize
	var vidOK, vidTotal, srtOK, srtTotal, exOK, exTotal, exSkipped int
	for _, r := range results {
		switch {
		case strings.HasPrefix(r.description, "Video:"):
			vidTotal++
			if r.err == nil {
				vidOK++
			}
		case strings.HasPrefix(r.description, "Subtitle:"):
			srtTotal++
			if r.err == nil {
				srtOK++
			}
		case strings.HasPrefix(r.description, "Exercise:"):
			exTotal++
			if r.skipped {
				exSkipped++
			} else if r.err == nil {
				exOK++
			}
		}
	}

	fmt.Printf("\n  Downloaded %d/%d videos, %d/%d subtitles, %d/%d exercise files",
		vidOK, vidTotal, srtOK, srtTotal, exOK, exTotal)
	if exSkipped > 0 {
		fmt.Printf(" (%d skipped)", exSkipped)
	}
	fmt.Println()

	if vidOK < vidTotal {
		return fmt.Errorf("failed to download %d/%d videos", vidTotal-vidOK, vidTotal)
	}
	return nil
}

func (d *Downloader) downloadAll(ctx context.Context, jobs []downloadJob) []downloadResult {
	results := make([]downloadResult, len(jobs))
	jobChan := make(chan int, len(jobs))
	var wg sync.WaitGroup

	// Track completed jobs for progress
	var completed atomic.Int64
	total := int64(len(jobs))

	// Worker pool
	for w := 0; w < d.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobChan {
				select {
				case <-ctx.Done():
					results[idx] = downloadResult{
						description: jobs[idx].description,
						err:         ctx.Err(),
						critical:    jobs[idx].critical,
					}
					continue
				default:
				}

				job := jobs[idx]
				var err error
				if job.content != "" {
					err = os.WriteFile(job.destPath, []byte(job.content), 0644)
				} else if !job.critical && d.authClient != nil {
					err = d.downloadFileWithClient(ctx, d.authClient, job.url, job.destPath, job.description)
				} else {
					err = d.downloadWithRetry(ctx, job, 5, job.critical)
				}

				done := completed.Add(1)
				if err == nil {
					slog.Info("Downloaded", "file", job.description, "progress", fmt.Sprintf("%d/%d", done, total))
				}

				skipped := false
				if err != nil && !job.critical {
					if isDNSError(err) {
						skipped = true
						slog.Warn("Skipped exercise file (dead CDN)", "file", job.description, "error", err)
					} else {
						slog.Warn("Failed to download exercise file", "file", job.description, "error", err)
					}
					err = nil // non-critical, don't propagate
				}

				results[idx] = downloadResult{
					description: job.description,
					err:         err,
					critical:    job.critical,
					skipped:     skipped,
				}
			}
		}()
	}

	// Enqueue all jobs
	for i := range jobs {
		jobChan <- i
	}
	close(jobChan)

	wg.Wait()
	return results
}

func (d *Downloader) downloadFileWithClient(ctx context.Context, client *http.Client, rawURL, destPath, description string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if d.csrfToken != "" {
		req.Header.Set("Csrf-Token", d.csrfToken)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("HTTP 404: resource not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Download to temp file first, then rename
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("  %s", description)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100),
	)

	d.progressMu.Lock()
	_, copyErr := io.Copy(io.MultiWriter(f, bar), resp.Body)
	d.progressMu.Unlock()
	if copyErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing file: %w", copyErr)
	}

	f.Close()
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0"

func (d *Downloader) downloadWithRetry(ctx context.Context, job downloadJob, maxRetries int, critical bool) error {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := d.downloadFile(ctx, job.url, job.destPath, job.description)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry DNS errors or 404s — the resource is gone
		if isDNSError(err) || isHTTP404(err) {
			if !critical {
				return err
			}
			slog.Error("Download failed (no retry)", "file", job.description, "error", err)
			return err
		}

		if attempt < maxRetries {
			slog.Warn("Download failed, retrying", "file", job.description, "attempt", attempt, "error", err)
		}
	}
	return fmt.Errorf("download failed after %d attempts: %w", maxRetries, lastErr)
}

func (d *Downloader) downloadFile(ctx context.Context, fileURL, destPath, description string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("HTTP 404: resource not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Download to temp file first, then rename
	tmpPath := destPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("  %s", description)),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(40),
		progressbar.OptionThrottle(100),
	)

	d.progressMu.Lock()
	_, copyErr := io.Copy(io.MultiWriter(f, bar), resp.Body)
	d.progressMu.Unlock()
	if copyErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing file: %w", copyErr)
	}

	f.Close()
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

func isDNSError(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func isHTTP404(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "HTTP 404") ||
		strings.Contains(err.Error(), "not found")
}
