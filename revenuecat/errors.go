package revenuecat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Error is a non-2xx response from the RevenueCat REST API v2.
type Error struct {
	Status    int
	Type      string
	Param     string
	Message   string
	DocURL    string
	Retryable bool
	BackoffMs int
	RequestID string
}

func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "revenuecat: HTTP %d", e.Status)
	if e.Type != "" {
		b.WriteString(" ")
		b.WriteString(e.Type)
	}

	if e.Message != "" {
		b.WriteString(": ")
		b.WriteString(e.Message)
	}

	var details []string
	if e.Param != "" {
		details = append(details, "param: "+e.Param)
	}
	if e.Retryable {
		if e.BackoffMs > 0 {
			details = append(details, fmt.Sprintf("retryable after %dms", e.BackoffMs))
		} else {
			details = append(details, "retryable")
		}
	}
	if e.DocURL != "" {
		details = append(details, "doc: "+e.DocURL)
	}
	if e.RequestID != "" {
		details = append(details, "request: "+e.RequestID)
	}

	if len(details) > 0 {
		b.WriteString(" (" + strings.Join(details, ", ") + ")")
	}

	return b.String()
}

// ResponseTooLargeError is returned instead of a body over MaxResponseBytes.
type ResponseTooLargeError struct {
	Limit int
}

func (e *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("revenuecat: response is larger than %d KB; use a smaller limit or narrower filters", e.Limit>>10)
}

const maxErrorSnippet = 200

// parseError reads RevenueCat's v2 top-level error object, falling back to the
// HTTP status text and a truncated body snippet when the payload is not one.
func parseError(resp *http.Response, body []byte) *Error {
	reqID := resp.Header.Get("X-Request-Id")
	if reqID == "" {
		reqID = resp.Header.Get("Request-Id")
	}

	e := &Error{Status: resp.StatusCode, RequestID: reqID}

	var envelope struct {
		Object    string `json:"object"`
		Type      string `json:"type"`
		Param     string `json:"param"`
		Message   string `json:"message"`
		DocURL    string `json:"doc_url"`
		Retryable bool   `json:"retryable"`
		BackoffMs int    `json:"backoff_ms"`
	}

	if json.Unmarshal(body, &envelope) == nil && (envelope.Object == "error" || envelope.Type != "" || envelope.Message != "") {
		e.Type = envelope.Type
		e.Param = envelope.Param
		e.Message = envelope.Message
		e.DocURL = envelope.DocURL
		e.Retryable = envelope.Retryable
		e.BackoffMs = envelope.BackoffMs
		return e
	}

	snippet := strings.TrimSpace(string(body))
	if len(snippet) > maxErrorSnippet {
		snippet = strings.ToValidUTF8(snippet[:maxErrorSnippet], "") + "..."
	}

	e.Message = http.StatusText(resp.StatusCode)
	if snippet != "" {
		e.Message += ": " + snippet
	}

	return e
}
