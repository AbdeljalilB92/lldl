// Package errors defines typed errors used across all features.
// Every feature uses these instead of raw fmt.Errorf wrapping,
// so callers can type-switch on error kind for targeted handling.
package errors

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// AuthError indicates a failure during authentication or token validation.
type AuthError struct {
	Cause error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("auth error: %v", e.Cause)
}

func (e *AuthError) Unwrap() error {
	return e.Cause
}

// ParseError indicates a failure when parsing an API response or structured data.
type ParseError struct {
	Source string
	Cause  error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error (%s): %v", e.Source, e.Cause)
}

func (e *ParseError) Unwrap() error {
	return e.Cause
}

// NetworkError indicates an HTTP or network-level failure.
// Retryable distinguishes transient failures from permanent ones.
type NetworkError struct {
	URL       string
	Cause     error
	Retryable bool
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error (%s): %v", e.URL, e.Cause)
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// ConfigError indicates a failure loading or saving configuration.
type ConfigError struct {
	Path  string
	Cause error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config error (%s): %v", e.Path, e.Cause)
}

func (e *ConfigError) Unwrap() error {
	return e.Cause
}

// ValidationError indicates invalid input at a feature boundary.
// This is a leaf error with no underlying cause.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error (%s): %s", e.Field, e.Message)
}

// IsRetryable returns true if the error is a NetworkError marked as retryable,
// or wraps one.
func IsRetryable(err error) bool {
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return netErr.Retryable
	}
	return false
}

// IsDNSError returns true if the error chain contains a DNS resolution failure,
// indicated by "no such host" or "lookup" in any wrapped error message.
func IsDNSError(err error) bool {
	for err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no such host") || strings.Contains(msg, "lookup") {
			// Confirm it's an actual DNS error from the net package, not a false match
			// in user-facing messages.
			var dnsErr *net.DNSError
			if errors.As(err, &dnsErr) {
				return true
			}
		}
		err = errors.Unwrap(err)
	}
	return false
}
