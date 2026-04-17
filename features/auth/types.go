package auth

import (
	"fmt"

	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
)

// Result contains the authenticated session data needed by downstream
// features (course fetching, video resolution, exercise file download).
type Result struct {
	Client         sharedhttp.AuthenticatedClient
	CSRFToken      string
	EnterpriseHash string
}

// String returns a safe string representation with sensitive fields redacted.
func (r Result) String() string {
	csrf := "REDACTED"
	if len(r.CSRFToken) > 0 {
		csrf = "SET"
	}
	ehash := "REDACTED"
	if len(r.EnterpriseHash) > 0 {
		ehash = "SET"
	}
	return "auth.Result{CSRFToken: " + csrf + ", EnterpriseHash: " + ehash + "}"
}

// TokenInfo represents a validated token with its status.
type TokenInfo struct {
	Token string
	Valid bool
}

// String returns a safe string representation with the token redacted.
func (ti TokenInfo) String() string {
	token := "REDACTED"
	if len(ti.Token) > 0 {
		token = "SET"
	}
	return "auth.TokenInfo{Token: " + token + ", Valid: " + fmt.Sprintf("%v", ti.Valid) + "}"
}
