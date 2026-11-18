package revenuecat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dvcrn/mcp-server-revenuecat-org/internal/httpclient"
)

func TestClientGetBuildsRequestAndHeaders(t *testing.T) {
	var (
		gotMethod      string
		gotEscapedPath string
		gotRawQuery    string
		gotAuth        string
		gotAccept      string
		gotUA          string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotEscapedPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"object":"customer","id":"$RCAnonymousID:abc/def"}`))
	}))
	defer srv.Close()

	q := NewQuery()
	if err := q.AddStrings("expand", []string{"attributes", "items.package"}); err != nil {
		t.Fatal(err)
	}
	q.SetInt("limit", 20)

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	body, err := c.Get(
		t.Context(),
		"/v2/projects/{project_id}/customers/{customer_id}",
		[]string{"projects", "proj1ab2c3d4", "customers", "$RCAnonymousID:abc/def"},
		q.Values(),
	)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Fatalf("method = %q, want GET", gotMethod)
	}
	if want := "/v2/projects/proj1ab2c3d4/customers/$RCAnonymousID:abc%2Fdef"; gotEscapedPath != want {
		t.Fatalf("escaped path = %q, want %q", gotEscapedPath, want)
	}
	if want := "expand=attributes&expand=items.package&limit=20"; gotRawQuery != want {
		t.Fatalf("query = %q, want %q", gotRawQuery, want)
	}
	if gotAuth != "Bearer sk_dummy_projectA" {
		t.Fatalf("Authorization = %q, want Bearer sk_dummy_projectA", gotAuth)
	}
	if gotAccept != "application/json" {
		t.Fatalf("Accept = %q, want application/json", gotAccept)
	}
	if gotUA != "mcp-server-revenuecat-org" {
		t.Fatalf("User-Agent = %q, want mcp-server-revenuecat-org", gotUA)
	}
	if !strings.Contains(string(body), `"object":"customer"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestClientPostAndDelete(t *testing.T) {
	var (
		gotMethod      string
		gotEscapedPath string
		gotContentType string
		gotBody        string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotEscapedPath = r.URL.EscapedPath()
		gotContentType = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"object":"entitlement","id":"entl_dummy"}`))
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))

	_, err := c.Post(
		t.Context(),
		"/v2/projects/{project_id}/entitlements",
		[]string{"projects", "proj1ab2c3d4", "entitlements"},
		nil,
		map[string]any{"lookup_key": "pro", "display_name": "Pro"},
	)
	if err != nil {
		t.Fatalf("Post with body: %v", err)
	}
	if gotMethod != http.MethodPost || gotEscapedPath != "/v2/projects/proj1ab2c3d4/entitlements" {
		t.Fatalf("got %s %s", gotMethod, gotEscapedPath)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody != `{"display_name":"Pro","lookup_key":"pro"}` {
		t.Fatalf("body = %q", gotBody)
	}

	_, err = c.Post(
		t.Context(),
		"/v2/projects/{project_id}/subscriptions/{subscription_id}/actions/cancel",
		[]string{"projects", "proj1ab2c3d4", "subscriptions", "sub_dummy", "actions", "cancel"},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("Post without body: %v", err)
	}
	if gotMethod != http.MethodPost || gotContentType != "" || gotBody != "" {
		t.Fatalf("bodyless Post sent Content-Type=%q body=%q", gotContentType, gotBody)
	}

	// Typed-nil pointer must fail rather than sending literal "null".
	var typedNil *struct{ Name string }
	if _, err := c.Post(t.Context(), "/v2/test", []string{"projects"}, nil, typedNil); err == nil {
		t.Fatal("Post with typed-nil pointer succeeded, want error")
	}

	_, err = c.Delete(
		t.Context(),
		"/v2/projects/{project_id}/entitlements/{entitlement_id}",
		[]string{"projects", "proj1ab2c3d4", "entitlements", "entl_dummy"},
	)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotMethod != http.MethodDelete || gotEscapedPath != "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy" {
		t.Fatalf("Delete got %s %s", gotMethod, gotEscapedPath)
	}
}

func TestClientReadOnlyContextBlocksWrites(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("unexpected %s request reached server in read-only context", r.Method)
		}
		w.Write([]byte(`{"object":"list","items":[]}`))
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	ctx := WithReadOnly(t.Context())

	if _, err := c.Get(ctx, "/v2/projects", []string{"projects"}, nil); err != nil {
		t.Fatalf("Get in read-only context failed: %v", err)
	}
	if _, err := c.Post(ctx, "/v2/projects", []string{"projects"}, nil, map[string]any{"a": 1}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Post in read-only context err = %v, want ErrReadOnly", err)
	}
	if _, err := c.Delete(ctx, "/v2/projects/proj1ab2c3d4", []string{"projects", "proj1ab2c3d4"}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete in read-only context err = %v, want ErrReadOnly", err)
	}
}

func TestClientRejectsDangerousSegments(t *testing.T) {
	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL("http://127.0.0.1:1"))

	bad := [][]string{
		nil,
		{"projects", ""},
		{"projects", "."},
		{"projects", ".."},
		{"projects", "proj1ab2c3d4\x00"},
		{"projects", "proj1ab2c3d4\n"},
		{"projects", string([]byte{0xff})},
	}

	for _, segs := range bad {
		if _, err := c.Get(t.Context(), "/v2/test", segs, nil); err == nil {
			t.Fatalf("Get(%q) succeeded, want error", segs)
		}
	}
}

func TestClientAllowsUTF8AndSpecialCustomerIDs(t *testing.T) {
	var gotEscaped string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscaped = r.URL.EscapedPath()
		w.Write([]byte(`{"object":"customer"}`))
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	if _, err := c.Get(
		t.Context(),
		"/v2/projects/{project_id}/customers/{customer_id}",
		[]string{"projects", "proj1ab2c3d4", "customers", "user+tag@example.com?x=1#frag"},
		nil,
	); err != nil {
		t.Fatalf("Get: %v", err)
	}

	if want := "/v2/projects/proj1ab2c3d4/customers/user+tag@example.com%3Fx=1%23frag"; gotEscaped != want {
		t.Fatalf("escaped path = %q, want %q", gotEscaped, want)
	}
}

func TestClientRedactsPIIAndSecretsFromErrorsAndFormatting(t *testing.T) {
	c := New(httpclient.New(), "sk_dummy_supersecret", WithBaseURL("http://127.0.0.1:1"))

	if got := fmt.Sprintf("%s %v %#v", c, c, c); strings.Contains(got, "sk_dummy_supersecret") {
		t.Fatalf("formatted client leaked key: %s", got)
	}

	q := NewQuery()
	_ = q.SetString("search", "alice@example.com")
	_, err := c.Get(
		t.Context(),
		"/v2/projects/{project_id}/customers/{customer_id}",
		[]string{"projects", "proj1ab2c3d4", "customers", "alice@example.com"},
		q.Values(),
	)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
	if strings.Contains(err.Error(), "alice@example.com") || strings.Contains(err.Error(), "sk_dummy_supersecret") {
		t.Fatalf("transport error leaked PII or secret: %v", err)
	}
}

func TestClientParsesErrorEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Request-Id", "req_dummy_123")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"object":"error","type":"parameter_error","param":"expand","message":"'foo' is not valid","doc_url":"https://errors.rev.cat/parameter-error","retryable":true,"backoff_ms":1500}`))
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	_, err := c.Get(t.Context(), "/v2/projects/{project_id}/offerings", []string{"projects", "proj1ab2c3d4", "offerings"}, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T (%v), want *Error", err, err)
	}
	if apiErr.Status != 400 || apiErr.Type != "parameter_error" || apiErr.Param != "expand" || !apiErr.Retryable || apiErr.BackoffMs != 1500 || apiErr.RequestID != "req_dummy_123" {
		t.Fatalf("unexpected parsed error: %+v", apiErr)
	}
	if !strings.Contains(apiErr.Error(), "retryable after 1500ms") {
		t.Fatalf("error string missing backoff: %s", apiErr.Error())
	}
}

func TestClientResponseTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(bytes.Repeat([]byte("a"), MaxResponseBytes+1))
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	_, err := c.Get(t.Context(), "/v2/projects", []string{"projects"}, nil)
	var largeErr *ResponseTooLargeError
	if !errors.As(err, &largeErr) {
		t.Fatalf("err = %v, want *ResponseTooLargeError", err)
	}
}

func TestClientNonJSONErrorTruncatesAndSanitizesUTF8(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		payload := append([]byte("<html>gateway failure "), 0xff, 0xfe)
		payload = append(payload, bytes.Repeat([]byte("x"), 400)...)
		w.Write(payload)
	}))
	defer srv.Close()

	c := New(httpclient.New(), "sk_dummy_projectA", WithBaseURL(srv.URL))
	_, err := c.Get(t.Context(), "/v2/projects", []string{"projects"}, nil)
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T (%v), want *Error", err, err)
	}
	if apiErr.Status != http.StatusBadGateway || !strings.HasSuffix(apiErr.Message, "...") || len(apiErr.Message) > 260 {
		t.Fatalf("unexpected non-JSON error message (%d bytes): %q", len(apiErr.Message), apiErr.Message)
	}
}

func TestValidators(t *testing.T) {
	if !ValidProjectID("proj1ab2c3d4") || ValidProjectID("acct_123") || ValidProjectID("proj") {
		t.Fatal("unexpected ValidProjectID result")
	}
	if !ValidSecretKey("sk_dummy_projectA") || ValidSecretKey("appl_dummy") || ValidSecretKey("sk_") || ValidSecretKey("sk_bad key") {
		t.Fatal("unexpected ValidSecretKey result")
	}
}
