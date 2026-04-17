package auth

import sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"

// Result contains the authenticated session data needed by downstream
// features (course fetching, video resolution, exercise file download).
type Result struct {
	Client         sharedhttp.AuthenticatedClient
	CSRFToken      string
	EnterpriseHash string
}

// TokenInfo represents a validated token with its status.
type TokenInfo struct {
	Token string
	Valid bool
}
