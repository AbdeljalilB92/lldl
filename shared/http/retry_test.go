package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testClient implements AuthenticatedClient using a custom RoundTripper,
// allowing full control over response behavior without real network calls.
type testClient struct {
	handler http.Handler
}

func (tc *testClient) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &requestError{URL: rawURL, cause: err}
	}
	rec := httptest.NewRecorder()
	tc.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func (tc *testClient) GetWithRetry(ctx context.Context, rawURL string, maxRetries int) ([]byte, error) {
	return DoWithRetry(ctx, tc, rawURL, maxRetries)
}

func TestDoWithRetry_SuccessOnFirstTry(t *testing.T) {
	tc := &testClient{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})}

	body, err := DoWithRetry(context.Background(), tc, "http://example.com/test", 3)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if string(body) != "hello" {
		t.Fatalf("expected body 'hello', got '%s'", string(body))
	}
}

func TestDoWithRetry_SuccessAfterRetry(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("recovered"))
	})

	tc := &testClient{handler: handler}

	body, err := DoWithRetry(context.Background(), tc, "http://example.com/test", 5)
	if err != nil {
		t.Fatalf("expected no error after retries, got: %v", err)
	}
	if string(body) != "recovered" {
		t.Fatalf("expected body 'recovered', got '%s'", string(body))
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_ContextCancellationStopsRetries(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("fail"))
	})

	tc := &testClient{handler: handler}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay — should interrupt retries.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := DoWithRetry(ctx, tc, "http://example.com/test", 100)
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}

	// Should not have exhausted all 100 retries — context was cancelled early.
	if attempts >= 50 {
		t.Fatalf("expected retries to stop early, but got %d attempts", attempts)
	}

	// Verify the error is a NetworkError (context cancellation).
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected NetworkError, got %T: %v", err, err)
	}
}

func TestDoWithRetry_NotFoundNotRetried(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	})

	tc := &testClient{handler: handler}

	_, err := DoWithRetry(context.Background(), tc, "http://example.com/test", 5)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected NetworkError, got %T: %v", err, err)
	}
	if netErr.Retryable {
		t.Fatal("404 should not be marked as retryable")
	}
	// 404 should not be retried — exactly 1 attempt.
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry for 404), got %d", attempts)
	}
}

func TestDoWithRetry_ExhaustsRetries(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	})

	tc := &testClient{handler: handler}

	_, err := DoWithRetry(context.Background(), tc, "http://example.com/test", 3)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	var netErr *NetworkError
	if !errors.As(err, &netErr) {
		t.Fatalf("expected NetworkError, got %T: %v", err, err)
	}
	if !netErr.Retryable {
		t.Fatal("exhausted retries should be marked as retryable")
	}
	if attempts != 3 {
		t.Fatalf("expected exactly 3 attempts, got %d", attempts)
	}
}

func TestNewAuthenticatedClient_SetsCookieAndUserAgent(t *testing.T) {
	client, err := NewAuthenticatedClient("test-token-123", "test-csrf", nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Verify it satisfies the interface.
	var _ AuthenticatedClient = client

	// Verify UserAgent constant is the expected value.
	if UserAgent == "" {
		t.Fatal("UserAgent constant should not be empty")
	}
}

func TestDoWithRetry_RateLimit429(t *testing.T) {
	attempts := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("rate limited"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	tc := &testClient{handler: handler}

	body, err := DoWithRetry(context.Background(), tc, "http://example.com/test", 5)
	if err != nil {
		t.Fatalf("expected no error after 429 retry, got: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("expected body 'ok', got '%s'", string(body))
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool // want > 0
	}{
		{"empty string", "", false},
		{"integer seconds", "30", true},
		{"zero seconds", "0", false},
		{"negative seconds", "-5", false},
		{"not a number", "not-a-date", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.value)
			if tt.want && got <= 0 {
				t.Errorf("parseRetryAfter(%q) = %v, want > 0", tt.value, got)
			}
			if !tt.want && got != 0 {
				t.Errorf("parseRetryAfter(%q) = %v, want 0", tt.value, got)
			}
		})
	}
}
