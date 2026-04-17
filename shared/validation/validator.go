// Package validation provides input validation helpers for feature boundaries.
// All validation returns shared/errors.ValidationError so callers can type-switch
// on error kind without string matching on messages.
package validation

import (
	"net/url"
	"strings"

	"github.com/AbdeljalilB92/lldl/shared/errors"
)

const minTokenLength = 10

// ValidateCourseURL parses a raw URL string, confirms it targets
// linkedin.com/learning, and extracts the course slug from the path.
// It uses url.Parse exclusively — no regex, no string interpolation into URLs.
func ValidateCourseURL(rawURL string) (string, error) {
	if rawURL == "" {
		return "", &errors.ValidationError{Field: "url", Message: "course URL must not be empty"}
	}

	// LinkedIn Learning URLs commonly omit the scheme. Default to https.
	normalized := rawURL
	if !strings.HasPrefix(normalized, "http://") && !strings.HasPrefix(normalized, "https://") {
		normalized = "https://" + normalized
	}

	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", &errors.ValidationError{Field: "url", Message: "could not parse URL: " + err.Error()}
	}

	host := strings.ToLower(parsed.Host)
	if host != "www.linkedin.com" && host != "linkedin.com" {
		return "", &errors.ValidationError{Field: "url", Message: "URL host must be linkedin.com, got " + parsed.Host}
	}

	// Expect path prefix /learning/<slug>
	path := strings.TrimPrefix(parsed.Path, "/")
	if !strings.HasPrefix(path, "learning/") {
		return "", &errors.ValidationError{Field: "url", Message: "URL path must start with /learning/, got " + parsed.Path}
	}

	rest := strings.TrimPrefix(path, "learning/")
	// Remove trailing slash and any remaining path segments
	if idx := strings.Index(rest, "/"); idx >= 0 {
		rest = rest[:idx]
	}

	slug := strings.TrimSpace(rest)
	if slug == "" {
		return "", &errors.ValidationError{Field: "url", Message: "course slug is missing from URL path"}
	}

	return slug, nil
}

// ValidateToken checks that an auth token is non-empty and meets a minimum
// length threshold. This is a fast pre-flight check, not a cryptographic
// validation — actual token validity is confirmed by the auth feature.
func ValidateToken(token string) error {
	if token == "" {
		return &errors.ValidationError{Field: "token", Message: "auth token must not be empty"}
	}
	if len(token) < minTokenLength {
		return &errors.ValidationError{Field: "token", Message: "auth token is too short (minimum 10 characters)"}
	}
	return nil
}

// ValidateOutputPath checks that a download output path is non-empty.
func ValidateOutputPath(path string) error {
	if path == "" {
		return &errors.ValidationError{Field: "output_path", Message: "output path must not be empty"}
	}
	return nil
}
