// Package httpclient builds the outbound HTTP client for the current platform.
package httpclient

import (
	"errors"
	"net/http"
	"time"
)

// ErrRedirect is returned instead of following 3xx responses so secret keys
// never leave the configured RevenueCat API host.
var ErrRedirect = errors.New("httpclient: redirects are not followed")

// New returns an outbound client backed by fetch on Workers and net/http elsewhere.
func New() *http.Client {
	client := newClient()
	client.Timeout = 10 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return ErrRedirect
	}
	return client
}
