package validation

import (
	"errors"
	"testing"

	apperrors "github.com/AbdeljalilB92/lldl/shared/errors"
)

func TestValidateCourseURL(t *testing.T) {
	tests := []struct {
		name      string
		rawURL    string
		wantSlug  string
		wantErr   bool
		errField  string
		errMsgSub string
	}{
		{
			name:     "full https URL",
			rawURL:   "https://www.linkedin.com/learning/learning-go-123",
			wantSlug: "learning-go-123",
		},
		{
			name:     "full http URL",
			rawURL:   "http://www.linkedin.com/learning/learning-go-123",
			wantSlug: "learning-go-123",
		},
		{
			name:     "no scheme auto-prepends https",
			rawURL:   "www.linkedin.com/learning/learning-go-123",
			wantSlug: "learning-go-123",
		},
		{
			name:     "no www prefix",
			rawURL:   "https://linkedin.com/learning/learning-go-123",
			wantSlug: "learning-go-123",
		},
		{
			name:     "URL with query parameters",
			rawURL:   "https://www.linkedin.com/learning/learning-go-123?foo=bar&baz=1",
			wantSlug: "learning-go-123",
		},
		{
			name:     "URL with trailing slash",
			rawURL:   "https://www.linkedin.com/learning/learning-go-123/",
			wantSlug: "learning-go-123",
		},
		{
			name:      "empty string",
			rawURL:    "",
			wantErr:   true,
			errField:  "url",
			errMsgSub: "must not be empty",
		},
		{
			name:      "invalid domain",
			rawURL:    "https://evil.com/learning/learning-go-123",
			wantErr:   true,
			errField:  "url",
			errMsgSub: "must be linkedin.com",
		},
		{
			name:      "missing slug",
			rawURL:    "https://www.linkedin.com/learning/",
			wantErr:   true,
			errField:  "url",
			errMsgSub: "slug is missing",
		},
		{
			name:      "missing learning prefix in path",
			rawURL:    "https://www.linkedin.com/learning-go-123",
			wantErr:   true,
			errField:  "url",
			errMsgSub: "must start with /learning/",
		},
		{
			name:     "numeric slug",
			rawURL:   "https://www.linkedin.com/learning/12345",
			wantSlug: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug, err := ValidateCourseURL(tt.rawURL)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var ve *apperrors.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected ValidationError, got %T: %v", err, err)
				}
				if ve.Field != tt.errField {
					t.Errorf("error field = %q, want %q", ve.Field, tt.errField)
				}
				if tt.errMsgSub != "" && !contains(ve.Message, tt.errMsgSub) {
					t.Errorf("error message = %q, want substring %q", ve.Message, tt.errMsgSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slug != tt.wantSlug {
				t.Errorf("slug = %q, want %q", slug, tt.wantSlug)
			}
		})
	}
}

func TestValidateToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantErr   bool
		errMsgSub string
	}{
		{
			name:  "valid token",
			token: "AQEDAxuN_JwBOLAHAAYBAQEEAQEEAQEEAQEEAQEEAQEE",
		},
		{
			name:      "empty token",
			token:     "",
			wantErr:   true,
			errMsgSub: "must not be empty",
		},
		{
			name:      "too short token",
			token:     "abc",
			wantErr:   true,
			errMsgSub: "too short",
		},
		{
			name:  "exactly minimum length",
			token: "0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToken(tt.token)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var ve *apperrors.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected ValidationError, got %T: %v", err, err)
				}
				if ve.Field != "token" {
					t.Errorf("error field = %q, want %q", ve.Field, "token")
				}
				if !contains(ve.Message, tt.errMsgSub) {
					t.Errorf("error message = %q, want substring %q", ve.Message, tt.errMsgSub)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateOutputPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name: "valid path",
			path: "/tmp/downloads",
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name: "relative path",
			path: "downloads",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOutputPath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				var ve *apperrors.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected ValidationError, got %T: %v", err, err)
				}
				if ve.Field != "output_path" {
					t.Errorf("error field = %q, want %q", ve.Field, "output_path")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// contains reports whether sub is a substring of s.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
