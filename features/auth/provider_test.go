package auth

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"testing"

	sharederrors "github.com/AbdeljalilB92/lldl/shared/errors"
)

func TestNewLinkedInAuth_InterfaceCompliance(_ *testing.T) {
	var _ Provider = (*linkedinAuth)(nil)
	var _ Provider = NewLinkedInAuth()
}

// TestAuthenticate_Success verifies that a valid response returns the correct
// CSRF token and enterprise hash from the LinkedIn Learning homepage.
func TestAuthenticate_Success(t *testing.T) {
	jsessionID := "test-jsessionid-abc123"
	enterpriseHash := "enterprise-hash-42"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Set JSESSIONID cookie (LinkedIn's Java backend does this)
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: jsessionID,
			Path:  "/",
		})

		// Return a page body with enterprise hash but no trial link
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>` +
			`<script>data={"enterpriseProfileHash":"` + enterpriseHash + `"}</script>` +
			`</body></html>`))
	}))
	defer server.Close()

	provider := &linkedinAuth{baseURL: server.URL}
	result, err := provider.Authenticate(context.Background(), "valid-token-12345")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.CSRFToken != jsessionID {
		t.Errorf("expected CSRFToken %q, got %q", jsessionID, result.CSRFToken)
	}
	if result.EnterpriseHash != enterpriseHash {
		t.Errorf("expected EnterpriseHash %q, got %q", enterpriseHash, result.EnterpriseHash)
	}
	if result.Client == nil {
		t.Error("expected Client to be non-nil")
	}
}

// TestAuthenticate_Success_NoEnterprise verifies authentication succeeds
// when no enterprise hash is present (individual accounts).
func TestAuthenticate_Success_NoEnterprise(t *testing.T) {
	jsessionID := "jsession-individual-user"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: jsessionID,
			Path:  "/",
		})
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>Learning homepage without enterprise</body></html>`))
	}))
	defer server.Close()

	provider := &linkedinAuth{baseURL: server.URL}
	result, err := provider.Authenticate(context.Background(), "individual-user-token")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.CSRFToken != jsessionID {
		t.Errorf("expected CSRFToken %q, got %q", jsessionID, result.CSRFToken)
	}
	if result.EnterpriseHash != "" {
		t.Errorf("expected empty EnterpriseHash, got %q", result.EnterpriseHash)
	}
}

// TestAuthenticate_TrialLinkDetected verifies that a response containing the
// "Start free trial" button returns an AuthError.
func TestAuthenticate_TrialLinkDetected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "JSESSIONID",
			Value: "some-jsessionid",
			Path:  "/",
		})

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>` +
			`<a class="nav__button-tertiary">\n  Start free trial</a>` +
			`</body></html>`))
	}))
	defer server.Close()

	provider := &linkedinAuth{baseURL: server.URL}
	_, err := provider.Authenticate(context.Background(), "expired-token-12345")
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}

	// Verify the error chain contains an AuthError
	var authErr *sharederrors.AuthError
	if !stderrors.As(err, &authErr) {
		t.Errorf("expected AuthError in chain, got %T: %v", err, err)
	}
}

// TestAuthenticate_EmptyToken verifies that an empty token fails validation
// before any network call is made.
func TestAuthenticate_EmptyToken(t *testing.T) {
	provider := NewLinkedInAuth()

	_, err := provider.Authenticate(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}

	var valErr *sharederrors.ValidationError
	if !stderrors.As(err, &valErr) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

// TestAuthenticate_ShortToken verifies that a token below minimum length
// fails validation.
func TestAuthenticate_ShortToken(t *testing.T) {
	provider := NewLinkedInAuth()

	_, err := provider.Authenticate(context.Background(), "short")
	if err == nil {
		t.Fatal("expected error for short token, got nil")
	}

	var valErr *sharederrors.ValidationError
	if !stderrors.As(err, &valErr) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

// TestAuthenticate_JSessionIDMissing verifies that authentication fails
// when the server doesn't set a JSESSIONID cookie.
func TestAuthenticate_JSessionIDMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No JSESSIONID cookie set, no trial link in body
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body>Authenticated content</body></html>`))
	}))
	defer server.Close()

	provider := &linkedinAuth{baseURL: server.URL}
	_, err := provider.Authenticate(context.Background(), "valid-but-no-jsessionid-token")
	if err == nil {
		t.Fatal("expected error when JSESSIONID is missing, got nil")
	}

	var authErr *sharederrors.AuthError
	if !stderrors.As(err, &authErr) {
		t.Errorf("expected AuthError in chain, got %T: %v", err, err)
	}
}

// TestRegexTrialLink verifies the trial link regex matches the expected pattern
// from the LinkedIn Learning homepage.
func TestRegexTrialLink(t *testing.T) {
	tests := []struct {
		name  string
		input string
		match bool
	}{
		{
			name:  "trial link with newline",
			input: `<a class="nav__button-tertiary">\nStart free trial</a>`,
			match: true,
		},
		{
			name:  "trial link with carriage return and newline",
			input: `<a class="nav__button-tertiary">\r\nStart free trial</a>`,
			match: true,
		},
		{
			name:  "trial link on same line",
			input: `<a class="nav__button-tertiary">Start free trial</a>`,
			match: true,
		},
		{
			name:  "no trial link",
			input: `<html><body>Welcome to LinkedIn Learning</body></html>`,
			match: false,
		},
		{
			name:  "similar but different text",
			input: `<a class="nav__button-tertiary">Start learning</a>`,
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := regexTrialLink.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("regexTrialLink.MatchString() = %v, want %v", got, tt.match)
			}
		})
	}
}

// TestRegexEnterpriseProfileHash verifies the enterprise hash regex extraction.
func TestRegexEnterpriseProfileHash(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOk bool
	}{
		{
			name:   "hash present in JSON",
			input:  `{"enterpriseProfileHash":"abc-123-def"}`,
			want:   "abc-123-def",
			wantOk: true,
		},
		{
			name:   "empty hash",
			input:  `{"enterpriseProfileHash":""}`,
			want:   "",
			wantOk: true,
		},
		{
			name:   "no enterprise field",
			input:  `{"someOtherField":"value"}`,
			want:   "",
			wantOk: false,
		},
		{
			name:   "hash with special chars",
			input:  `{"enterpriseProfileHash":"urn:li:enterprise:12345"}`,
			want:   "urn:li:enterprise:12345",
			wantOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := regexEnterpriseProfileHash.FindStringSubmatch(tt.input)
			if tt.wantOk {
				if len(matches) < 2 {
					t.Fatalf("expected match, got none")
				}
				if matches[1] != tt.want {
					t.Errorf("got %q, want %q", matches[1], tt.want)
				}
			} else {
				if len(matches) >= 2 {
					t.Errorf("expected no match, got %q", matches[1])
				}
			}
		})
	}
}
