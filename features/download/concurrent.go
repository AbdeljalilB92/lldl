package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
	"github.com/schollz/progressbar/v3"
)

// Compile-time check: concurrentEngine satisfies the Engine interface.
var _ Engine = (*concurrentEngine)(nil)

// defaultMaxRetries is the number of attempts for transient download failures.
const defaultMaxRetries = 5

// concurrentEngine downloads files using a worker pool with configurable concurrency.
// All HTTP calls go through the injected AuthenticatedClient.
type concurrentEngine struct {
	concurrency int
	client      sharedhttp.AuthenticatedClient
}

// NewConcurrentEngine creates an Engine that downloads files concurrently
// using a worker pool of the given size. The client handles authentication.
func NewConcurrentEngine(concurrency int, client sharedhttp.AuthenticatedClient) Engine {
	if concurrency <= 0 {
		concurrency = 4
	}
	return &concurrentEngine{
		concurrency: concurrency,
		client:      client,
	}
}

// DownloadAll executes all jobs using a worker pool and returns results
// in the same order as the input jobs.
func (e *concurrentEngine) DownloadAll(ctx context.Context, jobs []Job) []Result {
	logger := logging.New("[Download]")
	results := make([]Result, len(jobs))
	if len(jobs) == 0 {
		return results
	}

	jobChan := make(chan int, len(jobs))
	var completed atomic.Int64
	total := int64(len(jobs))
	var wg sync.WaitGroup

	// Launch workers
	for w := 0; w < e.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobChan {
				job := jobs[idx]

				select {
				case <-ctx.Done():
					results[idx] = Result{
						Description: job.Description,
						Err:         ctx.Err(),
						Critical:    job.Critical,
					}
					continue
				default:
				}

				var err error
				if len(job.Content) > 0 {
					// Ensure parent directory exists for content (transcript) jobs.
					if dir := parentDir(job.DestPath); dir != "" {
						if mkdirErr := os.MkdirAll(dir, 0755); mkdirErr != nil {
							err = fmt.Errorf("creating parent directory: %w", mkdirErr)
						}
					}
					if err == nil {
						err = os.WriteFile(job.DestPath, job.Content, 0600)
					}
				} else {
					err = e.downloadWithRetry(ctx, job)
				}

				done := completed.Add(1)
				if err == nil {
					logger.Info("Downloaded", "file", job.Description, "progress", fmt.Sprintf("%d/%d", done, total))
				}

				skipped := false
				if err != nil && !job.Critical {
					if sharederrors.IsDNSError(err) {
						skipped = true
						logger.Warn("Skipped non-critical file (dead CDN)", "file", job.Description, "error", err)
					} else {
						logger.Warn("Failed to download non-critical file", "file", job.Description, "error", err)
					}
					// Non-critical failures don't propagate as errors.
					err = nil
				}

				results[idx] = Result{
					Description: job.Description,
					Err:         err,
					Critical:    job.Critical,
					Skipped:     skipped,
				}
			}
		}()
	}

	// Enqueue all job indices
	for i := range jobs {
		jobChan <- i
	}
	close(jobChan)

	wg.Wait()
	return results
}

// downloadWithRetry attempts the download up to defaultMaxRetries times.
// DNS errors and 404s are not retried since the resource is permanently unavailable.
func (e *concurrentEngine) downloadWithRetry(ctx context.Context, job Job) error {
	logger := logging.New("[Download]")
	var lastErr error
	for attempt := 1; attempt <= defaultMaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := e.downloadFile(ctx, job.URL, job.DestPath, job.Description)
		if err == nil {
			return nil
		}
		lastErr = err

		// Don't retry DNS errors or 404s — the resource is gone.
		if sharederrors.IsDNSError(err) || isHTTP404(err) {
			if !job.Critical {
				return err
			}
			logger.Error("Download failed (no retry)", "file", job.Description, "error", err)
			return err
		}

		if attempt < defaultMaxRetries {
			logger.Warn("Download failed, retrying", "file", job.Description, "attempt", attempt, "error", err)
			// Exponential backoff capped at 10s.
			delay := time.Duration(1<<uint(attempt)) * time.Second
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			time.Sleep(delay)
		}
	}
	return &sharederrors.NetworkError{URL: job.URL, Cause: lastErr, Retryable: true}
}

// downloadFile performs a single HTTP GET and writes the response body to destPath.
// Uses temp file + atomic rename to prevent partial files on failure.
func (e *concurrentEngine) downloadFile(ctx context.Context, fileURL, destPath, description string) error {
	resp, err := e.client.Get(ctx, fileURL)
	if err != nil {
		return &sharederrors.NetworkError{URL: fileURL, Cause: err, Retryable: sharederrors.IsRetryable(err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("HTTP 404: resource not found")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Ensure parent directory exists.
	if dir := parentDir(destPath); dir != "" {
		if mkdirErr := os.MkdirAll(dir, 0755); mkdirErr != nil {
			return fmt.Errorf("creating parent directory: %w", mkdirErr)
		}
	}

	// Download to temp file first, then rename for atomicity.
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
		progressbar.OptionThrottle(100*time.Millisecond),
	)

	_, copyErr := io.Copy(io.MultiWriter(f, bar), resp.Body)
	if copyErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing file: %w", copyErr)
	}

	f.Close()
	if renameErr := os.Rename(tmpPath, destPath); renameErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", renameErr)
	}

	return nil
}

// isHTTP404 checks if an error message indicates a 404 response.
func isHTTP404(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "HTTP 404") || strings.Contains(msg, "not found")
}

// parentDir returns the parent directory of a file path.
// Returns empty string for paths without a directory component.
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}
