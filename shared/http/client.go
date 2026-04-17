// Package http provides the authenticated HTTP client abstraction used by all features.
// Consumers import this package with an alias to avoid shadowing the stdlib:
//
//	import sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
package http

import (
	"context"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// UserAgent is the single shared User-Agent string used for all LinkedIn API requests.
// Deduplicated from the former duplicates in internal/client and internal/download.
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0"

// AuthenticatedClient is the interface for making authenticated HTTP requests.
// All features depend on this interface; the concrete implementation lives here.
type AuthenticatedClient interface {
	// Get performs a single GET request and returns the raw response.
	// The caller is responsible for closing the response body.
	Get(ctx context.Context, url string) (*http.Response, error)

	// GetWithRetry performs a GET request with exponential backoff and returns the body bytes.
	GetWithRetry(ctx context.Context, url string, maxRetries int) ([]byte, error)
}

// Compile-time check: unexported concrete type satisfies the public interface.
var _ AuthenticatedClient = (*authenticatedClient)(nil)

// authenticatedClient wraps an *http.Client with cookie-based authentication.
type authenticatedClient struct {
	client *http.Client
}

// userAgentTransport wraps an http.RoundTripper to inject the User-Agent header.
type userAgentTransport struct {
	base http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", UserAgent)
	return t.base.RoundTrip(req)
}

// csrfTransport wraps an http.RoundTripper to inject the csrf-token header
// required by LinkedIn's API on every authenticated request.
type csrfTransport struct {
	base      http.RoundTripper
	csrfToken string
}

func (t *csrfTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("csrf-token", t.csrfToken)
	return t.base.RoundTrip(req)
}

// NewAuthenticatedClient creates an AuthenticatedClient pre-configured with the
// li_at cookie for LinkedIn authentication, a User-Agent header, and the
// csrf-token header required by LinkedIn's API.
// An optional extraCookie (e.g. JSESSIONID) is added to the jar when non-nil.
func NewAuthenticatedClient(token string, csrfToken string, extraCookie *http.Cookie) (AuthenticatedClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, &newClientError{cause: err}
	}

	liAtCookie := &http.Cookie{
		Name:     "li_at",
		Value:    token,
		Path:     "/",
		Domain:   ".www.linkedin.com",
		Secure:   true,
		HttpOnly: true,
	}
	linkedinURL, err := url.Parse("https://www.linkedin.com")
	if err != nil {
		return nil, &newClientError{cause: err}
	}
	jar.SetCookies(linkedinURL, []*http.Cookie{liAtCookie})
	if extraCookie != nil {
		jar.SetCookies(linkedinURL, []*http.Cookie{extraCookie})
	}

	// Chain transports: csrf -> userAgent -> defaultTransport.
	// This ensures both headers are present on every request.
	var transport http.RoundTripper = http.DefaultTransport
	transport = &userAgentTransport{base: transport}
	if csrfToken != "" {
		transport = &csrfTransport{base: transport, csrfToken: csrfToken}
	}

	client := &http.Client{
		Jar:       jar,
		Timeout:   60 * time.Second,
		Transport: transport,
	}

	return &authenticatedClient{client: client}, nil
}

// Get performs a single GET request with context, returning the raw response.
func (c *authenticatedClient) Get(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, &requestError{URL: rawURL, cause: err}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &NetworkError{URL: rawURL, Cause: err, Retryable: isRetryableError(err)}
	}
	return resp, nil
}

// GetWithRetry performs a GET with exponential backoff, returning body bytes on success.
func (c *authenticatedClient) GetWithRetry(ctx context.Context, rawURL string, maxRetries int) ([]byte, error) {
	return DoWithRetry(ctx, c, rawURL, maxRetries)
}
