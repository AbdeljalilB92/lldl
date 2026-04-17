package auth

import (
	"net/http"

	sharedhttp "github.com/AbdeljalilB92/lldl/shared/http"
)

// userAgentTransport injects the shared User-Agent header into all requests,
// ensuring auth validation sees the same request fingerprint as subsequent
// API calls made through shared/http.AuthenticatedClient.
type userAgentTransport struct {
	base http.RoundTripper
}

// RoundTrip adds the User-Agent header before delegating to the base transport.
// The request is cloned to avoid mutating the original, per the http.RoundTripper contract.
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header.Set("User-Agent", sharedhttp.UserAgent)
	return t.base.RoundTrip(cloned)
}
