package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"
)

// DoWithRetry performs a GET request with exponential backoff (1s, 2s, 4s...).
// It returns the body bytes on success or a typed NetworkError on exhaustion.
// DNS errors and HTTP 404 responses are not retried.
// HTTP 429 (Too Many Requests) respects the Retry-After header when present.
func DoWithRetry(ctx context.Context, client AuthenticatedClient, rawURL string, maxRetries int) ([]byte, error) {
	if maxRetries < 1 {
		return nil, &NetworkError{URL: rawURL, Cause: fmt.Errorf("maxRetries must be >= 1, got %d", maxRetries), Retryable: false}
	}

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Respect context cancellation before each attempt.
		if err := ctx.Err(); err != nil {
			return nil, &NetworkError{URL: rawURL, Cause: err, Retryable: false}
		}

		resp, err := client.Get(ctx, rawURL)
		if err != nil {
			// DNS errors are terminal — the host doesn't exist, retrying won't help.
			if isDNSError(err) {
				return nil, &NetworkError{URL: rawURL, Cause: err, Retryable: false}
			}
			lastErr = err
			waitBeforeRetry(ctx, attempt, maxRetries, 0)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			waitBeforeRetry(ctx, attempt, maxRetries, 0)
			continue
		}

		// HTTP 404 is terminal — the resource doesn't exist.
		if resp.StatusCode == http.StatusNotFound {
			return nil, &NetworkError{URL: rawURL, Cause: errors.New("HTTP 404"), Retryable: false}
		}

		// HTTP 429 (rate limited) respects Retry-After header for backoff duration.
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			lastErr = errors.New("HTTP 429 Too Many Requests")
			waitBeforeRetry(ctx, attempt, maxRetries, retryAfter)
			continue
		}

		if resp.StatusCode >= 400 {
			lastErr = errors.New(resp.Status)
			waitBeforeRetry(ctx, attempt, maxRetries, 0)
			continue
		}

		return body, nil
	}

	return nil, &NetworkError{URL: rawURL, Cause: lastErr, Retryable: true}
}

// waitBeforeRetry applies exponential backoff (1s, 2s, 4s...) between attempts,
// returning early if the context is cancelled. When retryAfter > 0 (from a
// Retry-After header), that duration is used instead of the exponential backoff.
func waitBeforeRetry(ctx context.Context, attempt, maxRetries int, retryAfter time.Duration) {
	if attempt >= maxRetries {
		return
	}
	backoff := retryAfter
	if backoff <= 0 {
		backoff = time.Duration(1<<(attempt-1)) * time.Second
		// Cap computed backoff at 30 seconds; honor server-provided Retry-After.
		maxBackoff := 30 * time.Second
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
	select {
	case <-time.After(backoff):
	case <-ctx.Done():
	}
}

// parseRetryAfter parses the Retry-After HTTP header value.
// It supports both integer seconds (e.g. "30") and HTTP-date formats.
// Unparseable or negative values return 0 (fallback to exponential backoff).
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}
	// Try integer seconds first (most common for APIs).
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}
	// HTTP-date format is rare in API responses; try parsing as RFC 1123.
	if t, err := http.ParseTime(value); err == nil {
		remaining := time.Until(t)
		if remaining > 0 {
			return remaining
		}
		return 0
	}
	return 0
}

// isDNSError returns true if the error is a DNS resolution failure.
func isDNSError(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

// isRetryableError classifies transient network errors as retryable.
// DNS errors, context cancellation, and URL parsing errors are not retryable.
func isRetryableError(err error) bool {
	if isDNSError(err) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// Default: treat unknown network errors as retryable (transient).
	return true
}

// ---------------------------------------------------------------------------
// Typed errors (package-local, not depending on shared/errors to avoid circular)
// ---------------------------------------------------------------------------

// NetworkError indicates an HTTP or network-level failure during a request.
type NetworkError struct {
	URL       string
	Cause     error
	Retryable bool
}

func (e *NetworkError) Error() string {
	if e.Cause != nil {
		return "network error: " + e.URL + ": " + e.Cause.Error()
	}
	return "network error: " + e.URL
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// requestError indicates failure to construct an HTTP request.
type requestError struct {
	URL   string
	cause error
}

func (e *requestError) Error() string {
	return "request error: " + e.URL + ": " + e.cause.Error()
}

func (e *requestError) Unwrap() error {
	return e.cause
}

// newClientError indicates failure during AuthenticatedClient construction.
type newClientError struct {
	cause error
}

func (e *newClientError) Error() string {
	return "client creation error: " + e.cause.Error()
}

func (e *newClientError) Unwrap() error {
	return e.cause
}
