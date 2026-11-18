// Package revenuecat is an HTTP client for the RevenueCat REST API v2.
package revenuecat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// MaxResponseBytes is the largest response body the client returns.
	MaxResponseBytes = 1 << 20

	defaultBaseURL = "https://api.revenuecat.com"
	userAgent      = "mcp-server-revenuecat-org"
)

// ErrReadOnly is returned when Post or Delete is called with a context marked
// read-only via WithReadOnly.
var ErrReadOnly = errors.New("revenuecat: write operation refused in read-only mode")

type readOnlyKey struct{}

// WithReadOnly marks ctx so Client.Post and Client.Delete refuse to send requests.
func WithReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyKey{}, true)
}

func isReadOnly(ctx context.Context) bool {
	ro, _ := ctx.Value(readOnlyKey{}).(bool)
	return ro
}

var projectIDPattern = regexp.MustCompile(`^proj[A-Za-z0-9_-]+$`)

// Client calls the RevenueCat REST API v2 with a project-scoped secret key.
type Client struct {
	httpClient *http.Client
	key        string
	baseURL    string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the client at another host, such as an httptest server.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// New returns a client that authenticates with the given RevenueCat v2 secret key.
func New(httpClient *http.Client, key string, opts ...Option) *Client {
	c := &Client{httpClient: httpClient, key: key, baseURL: defaultBaseURL}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// String redacts the secret key when formatted.
func (c *Client) String() string { return "revenuecat.Client{[REDACTED]}" }

// GoString redacts the secret key when formatted with %#v.
func (c *Client) GoString() string { return "revenuecat.Client{[REDACTED]}" }

// Get sends an HTTP GET request to /v2/<segments...> and returns the raw JSON
// response body. endpoint is a static route template (such as
// "/v2/projects/{project_id}/customers/{customer_id}") used in transport errors
// so customer identifiers in path segments or query strings are not echoed.
func (c *Client) Get(ctx context.Context, endpoint string, segments []string, query url.Values) ([]byte, error) {
	return c.do(ctx, http.MethodGet, endpoint, segments, query, nil, "")
}

// Post sends an HTTP POST request to /v2/<segments...>. When body is non-nil it
// is JSON-encoded with Content-Type application/json; pass nil for bodyless
// action endpoints such as actions/cancel and actions/refund.
func (c *Client) Post(ctx context.Context, endpoint string, segments []string, query url.Values, body any) ([]byte, error) {
	if isReadOnly(ctx) {
		return nil, ErrReadOnly
	}

	var (
		reader      io.Reader
		contentType string
	)
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("revenuecat: encode request body for %s: %w", endpoint, err)
		}
		if bytes.Equal(payload, []byte("null")) {
			return nil, fmt.Errorf("revenuecat: %s: nil request body", endpoint)
		}
		reader = bytes.NewReader(payload)
		contentType = "application/json"
	}

	return c.do(ctx, http.MethodPost, endpoint, segments, query, reader, contentType)
}

// Delete sends an HTTP DELETE request to /v2/<segments...>.
func (c *Client) Delete(ctx context.Context, endpoint string, segments []string) ([]byte, error) {
	if isReadOnly(ctx) {
		return nil, ErrReadOnly
	}

	return c.do(ctx, http.MethodDelete, endpoint, segments, nil, nil, "")
}

func (c *Client) do(ctx context.Context, method, endpoint string, segments []string, query url.Values, body io.Reader, contentType string) ([]byte, error) {
	if len(segments) == 0 {
		return nil, errors.New("revenuecat: empty request path")
	}

	escaped := make([]string, len(segments))
	for i, seg := range segments {
		if err := checkSegment(seg); err != nil {
			return nil, fmt.Errorf("revenuecat: %s: %w", endpoint, err)
		}
		escaped[i] = url.PathEscape(seg)
	}

	target := c.baseURL + "/v2/" + strings.Join(escaped, "/")
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("revenuecat: build request for %s: %w", endpoint, err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.key)
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", userAgent)
	if contentType != "" {
		httpReq.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}

		return nil, fmt.Errorf("revenuecat: %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("revenuecat: read response for %s: %w", endpoint, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, parseError(resp, data[:min(len(data), MaxResponseBytes)])
	}

	if len(data) > MaxResponseBytes {
		return nil, &ResponseTooLargeError{Limit: MaxResponseBytes}
	}

	return data, nil
}

// checkSegment rejects empty, dot-traversal, invalid UTF-8, or control-character
// path segments before url.PathEscape runs (url.PathEscape leaves "." and ".." intact).
func checkSegment(seg string) error {
	if seg == "" || seg == "." || seg == ".." || !utf8.ValidString(seg) {
		return errors.New("invalid path segment")
	}

	for _, r := range seg {
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) {
			return errors.New("invalid control character in path segment")
		}
	}

	return nil
}

// ValidProjectID reports whether id is a RevenueCat proj... identifier.
func ValidProjectID(id string) bool {
	return projectIDPattern.MatchString(id)
}

// ValidSecretKey reports whether key has the RevenueCat API v2 secret key prefix
// (sk_) followed by printable non-whitespace ASCII characters.
func ValidSecretKey(key string) bool {
	rest, ok := strings.CutPrefix(key, "sk_")
	if !ok || rest == "" {
		return false
	}

	for i := 0; i < len(rest); i++ {
		if rest[i] < 0x21 || rest[i] > 0x7e {
			return false
		}
	}

	return true
}
