package auth

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"time"

	"github.com/AbdeljalilB92/lldl/shared/errors"
	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
	"github.com/AbdeljalilB92/lldl/shared/logging"
	"github.com/AbdeljalilB92/lldl/shared/validation"
)

// Compile-time guarantee that linkedinAuth satisfies Provider.
var _ Provider = (*linkedinAuth)(nil)

const (
	// defaultBaseURL is the production LinkedIn Learning base URL.
	defaultBaseURL = "https://www.linkedin.com"
	// learningPath is appended to the base URL for token validation.
	learningPath = "/learning"
)

var (
	// regexTrialLink detects an unauthenticated or expired session on the
	// LinkedIn Learning homepage. The presence of "Start free trial" means
	// the li_at cookie is not recognized.
	regexTrialLink = regexp.MustCompile(`nav__button-tertiary.*\n?.\r?.*Start free trial`)

	// regexEnterpriseProfileHash extracts the enterprise profile hash embedded
	// in the page body. Enterprise users have this hash; individual accounts do not.
	regexEnterpriseProfileHash = regexp.MustCompile(`enterpriseProfileHash":"(.*?)"`)
)

// linkedinAuth implements Provider for LinkedIn's cookie-based authentication.
type linkedinAuth struct {
	baseURL string
	client  *http.Client
}

// NewLinkedInAuth creates an Provider that validates tokens against
// LinkedIn Learning and extracts CSRF/enterprise session data.
func NewLinkedInAuth() Provider {
	return &linkedinAuth{
		baseURL: defaultBaseURL,
	}
}

// Authenticate validates the given li_at token by hitting the LinkedIn Learning
// homepage. On success it returns an Result containing an authenticated
// HTTP client, the JSESSIONID-based CSRF token, and the enterprise hash (if any).
func (a *linkedinAuth) Authenticate(ctx context.Context, token string) (Result, error) {
	logger := logging.New("[Auth][Authenticate]")

	if err := validation.ValidateToken(token); err != nil {
		return Result{}, err
	}

	client, err := a.buildClient(token)
	if err != nil {
		return Result{}, &errors.AuthError{Cause: err}
	}
	a.client = client

	learningURL := a.baseURL + learningPath
	resp, err := a.doGet(ctx, learningURL)
	if err != nil {
		return Result{}, &errors.NetworkError{URL: learningURL, Cause: err, Retryable: true}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{}, &errors.NetworkError{URL: learningURL, Cause: err, Retryable: false}
	}
	unescaped := html.UnescapeString(string(body))

	if regexTrialLink.MatchString(unescaped) {
		logger.Warn("token validation failed: free trial link detected")
		return Result{}, &errors.AuthError{
			Cause: &errors.ValidationError{
				Field:   "token",
				Message: "token is expired or invalid (free trial page detected)",
			},
		}
	}

	csrfToken := a.extractJSessionID()
	if csrfToken == "" {
		return Result{}, &errors.AuthError{
			Cause: &errors.ValidationError{
				Field:   "token",
				Message: "JSESSIONID cookie not found; token may be invalid",
			},
		}
	}

	enterpriseHash := ""
	if m := regexEnterpriseProfileHash.FindStringSubmatch(unescaped); len(m) > 1 {
		enterpriseHash = m[1]
	}

	// Build the single authenticated client using the same cookie jar that
	// captured JSESSIONID during auth, and inject the CSRF token as a header.
	// This avoids the previous bug of creating a second client that lacked
	// both the JSESSIONID cookie and the csrf-token header.
	jsessionCookie := a.extractJSessionIDCookie()
	authedClient, err := sharedhttp.NewAuthenticatedClient(token, csrfToken, jsessionCookie)
	if err != nil {
		return Result{}, &errors.AuthError{Cause: err}
	}

	logger.Info("authentication successful",
		"enterprise", enterpriseHash != "",
		"csrf_set", csrfToken != "",
	)

	return Result{
		Client:         authedClient,
		CSRFToken:      csrfToken,
		EnterpriseHash: enterpriseHash,
	}, nil
}

// buildClient creates an *http.Client with a cookie jar pre-loaded with the
// li_at cookie. The jar is needed to capture JSESSIONID set by LinkedIn.
func (a *linkedinAuth) buildClient(token string) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, &errors.NetworkError{URL: a.baseURL, Cause: err, Retryable: false}
	}

	liAt := &http.Cookie{
		Name:     "li_at",
		Value:    token,
		Path:     "/",
		Domain:   ".www.linkedin.com",
		Secure:   true,
		HttpOnly: true,
	}
	parsedURL, err := url.Parse(a.baseURL)
	if err != nil {
		return nil, &errors.NetworkError{URL: a.baseURL, Cause: err, Retryable: false}
	}
	jar.SetCookies(parsedURL, []*http.Cookie{liAt})

	return &http.Client{
		Jar:     jar,
		Timeout: time.Duration(defaultClientTimeout) * time.Second,
		Transport: &userAgentTransport{
			base: http.DefaultTransport,
		},
	}, nil
}

// doGet performs a GET request with User-Agent header using the internal client.
func (a *linkedinAuth) doGet(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", sharedhttp.UserAgent)
	return a.client.Do(req)
}

// extractJSessionID reads the JSESSIONID cookie from the jar for the learning URL.
// JSESSIONID is set by LinkedIn's Java backend and serves as the CSRF token.
func (a *linkedinAuth) extractJSessionID() string {
	if a.client == nil || a.client.Jar == nil {
		return ""
	}
	learningURL := a.baseURL + learningPath
	parsedURL, err := url.Parse(learningURL)
	if err != nil {
		return ""
	}
	for _, c := range a.client.Jar.Cookies(parsedURL) {
		if c.Name == "JSESSIONID" {
			return c.Value
		}
	}
	return ""
}

// extractJSessionIDCookie returns the JSESSIONID cookie from the auth probe client's jar.
func (a *linkedinAuth) extractJSessionIDCookie() *http.Cookie {
	if a.client == nil || a.client.Jar == nil {
		return nil
	}
	learningURL := a.baseURL + learningPath
	parsedURL, err := url.Parse(learningURL)
	if err != nil {
		return nil
	}
	for _, c := range a.client.Jar.Cookies(parsedURL) {
		if c.Name == "JSESSIONID" {
			return c
		}
	}
	return nil
}

// defaultClientTimeout is the HTTP client timeout in seconds.
const defaultClientTimeout = 60
