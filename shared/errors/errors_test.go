package errors

import (
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestAuthError_Error(t *testing.T) {
	cause := fmt.Errorf("invalid token")
	err := &AuthError{Cause: cause}

	want := "auth error: invalid token"
	if got := err.Error(); got != want {
		t.Errorf("AuthError.Error() = %q, want %q", got, want)
	}
}

func TestAuthError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("invalid token")
	err := &AuthError{Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match the wrapped cause")
	}
}

func TestParseError_Error(t *testing.T) {
	cause := fmt.Errorf("unexpected EOF")
	err := &ParseError{Source: "course API response", Cause: cause}

	want := "parse error (course API response): unexpected EOF"
	if got := err.Error(); got != want {
		t.Errorf("ParseError.Error() = %q, want %q", got, want)
	}
}

func TestParseError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("unexpected EOF")
	err := &ParseError{Source: "course API response", Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match the wrapped cause")
	}
}

func TestNetworkError_Error(t *testing.T) {
	err := &NetworkError{URL: "https://example.com/api", Cause: fmt.Errorf("connection refused")}

	want := "network error (https://example.com/api): connection refused"
	if got := err.Error(); got != want {
		t.Errorf("NetworkError.Error() = %q, want %q", got, want)
	}
}

func TestNetworkError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	err := &NetworkError{URL: "https://example.com/api", Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match the wrapped cause")
	}
}

func TestConfigError_Error(t *testing.T) {
	err := &ConfigError{Path: "/home/user/.config/llcd/config.json", Cause: fmt.Errorf("permission denied")}

	want := "config error (/home/user/.config/llcd/config.json): permission denied"
	if got := err.Error(); got != want {
		t.Errorf("ConfigError.Error() = %q, want %q", got, want)
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("permission denied")
	err := &ConfigError{Path: "/home/user/config.json", Cause: cause}

	if !errors.Is(err, cause) {
		t.Error("errors.Is should match the wrapped cause")
	}
}

func TestValidationError_Error(t *testing.T) {
	err := &ValidationError{Field: "token", Message: "must not be empty"}

	want := "validation error (token): must not be empty"
	if got := err.Error(); got != want {
		t.Errorf("ValidationError.Error() = %q, want %q", got, want)
	}
}

func TestValidationError_NoUnwrap(t *testing.T) {
	err := &ValidationError{Field: "token", Message: "must not be empty"}

	// ValidationError has no Unwrap method, so it should not wrap anything.
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		t.Errorf("ValidationError.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "retryable network error",
			err:  &NetworkError{URL: "https://example.com", Cause: fmt.Errorf("timeout"), Retryable: true},
			want: true,
		},
		{
			name: "non-retryable network error",
			err:  &NetworkError{URL: "https://example.com", Cause: fmt.Errorf("403 forbidden"), Retryable: false},
			want: false,
		},
		{
			name: "auth error is not retryable",
			err:  &AuthError{Cause: fmt.Errorf("bad token")},
			want: false,
		},
		{
			name: "wrapped retryable network error",
			err:  fmt.Errorf("wrapper: %w", &NetworkError{URL: "https://example.com", Cause: fmt.Errorf("timeout"), Retryable: true}),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDNSError(t *testing.T) {
	dnsErr := &net.DNSError{
		Err:         "no such host",
		Name:        "api.linkedin.com",
		Server:      "8.8.8.8",
		IsTimeout:   false,
		IsTemporary: false,
		IsNotFound:  true,
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "direct DNS error",
			err:  dnsErr,
			want: true,
		},
		{
			name: "DNS error wrapped in NetworkError",
			err:  &NetworkError{URL: "https://api.linkedin.com", Cause: dnsErr, Retryable: true},
			want: true,
		},
		{
			name: "DNS error deeply wrapped",
			err:  fmt.Errorf("fetch failed: %w", &NetworkError{URL: "https://api.linkedin.com", Cause: dnsErr}),
			want: true,
		},
		{
			name: "non-DNS network error",
			err:  &NetworkError{URL: "https://example.com", Cause: fmt.Errorf("connection refused"), Retryable: false},
			want: false,
		},
		{
			name: "error with 'lookup' text but not a real DNS error",
			err:  fmt.Errorf("failed to lookup user in database"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDNSError(tt.err); got != tt.want {
				t.Errorf("IsDNSError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnwrapChain(t *testing.T) {
	root := fmt.Errorf("connection reset")
	networkErr := &NetworkError{URL: "https://example.com", Cause: root, Retryable: true}
	authErr := &AuthError{Cause: networkErr}

	// errors.Is should traverse the full chain.
	if !errors.Is(authErr, root) {
		t.Error("errors.Is should find root cause through AuthError -> NetworkError -> root")
	}
	if !errors.Is(authErr, networkErr) {
		t.Error("errors.Is should find intermediate NetworkError")
	}
	// As should extract the typed intermediate error.
	var netErr *NetworkError
	if !errors.As(authErr, &netErr) {
		t.Error("errors.As should extract NetworkError from chain")
	}
}
