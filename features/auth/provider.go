package auth

import "context"

// Provider validates a LinkedIn learning token and returns an authenticated
// session with CSRF token and optional enterprise hash.
type Provider interface {
	Authenticate(ctx context.Context, token string) (Result, error)
}
