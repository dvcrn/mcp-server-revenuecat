package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

const (
	expectedReadToolCount  = 29
	expectedWriteToolCount = 26
	expectedFullToolCount  = expectedReadToolCount + expectedWriteToolCount
)

func allWriteToolNames() []string {
	return []string{
		"create_customer",
		"delete_customer",
		"set_customer_attributes",
		"grant_customer_entitlement",
		"revoke_customer_entitlement",
		"assign_customer_offering",
		"transfer_customer",
		"cancel_subscription",
		"refund_subscription",
		"extend_subscription",
		"refund_purchase",
		"create_product",
		"delete_product",
		"create_entitlement",
		"update_entitlement",
		"delete_entitlement",
		"attach_entitlement_products",
		"detach_entitlement_products",
		"create_offering",
		"update_offering",
		"delete_offering",
		"create_offering_package",
		"update_package",
		"delete_package",
		"attach_package_products",
		"detach_package_products",
	}
}

func TestStdioServerFlagParsing(t *testing.T) {
	t.Parallel()

	cases := []struct {
		val     string
		wantRO  bool
		wantErr bool
	}{
		{val: "", wantRO: false},
		{val: "0", wantRO: false},
		{val: "false", wantRO: false},
		{val: "FALSE", wantRO: false},
		{val: "1", wantRO: true},
		{val: "true", wantRO: true},
		{val: " TRUE \n", wantRO: true},
		{val: "yes", wantErr: true},
		{val: "readonly", wantErr: true},
	}

	for _, tc := range cases {
		srv, err := StdioServer(func(k string) string {
			if k == EnvReadOnly {
				return tc.val
			}
			return ""
		})
		if tc.wantErr {
			if err == nil {
				t.Errorf("StdioServer(%q) succeeded, want error", tc.val)
			}
			continue
		}
		if err != nil {
			t.Errorf("StdioServer(%q) unexpected error: %v", tc.val, err)
			continue
		}
		want := New()
		if tc.wantRO {
			want = NewReadOnly()
		}
		if srv != want {
			t.Errorf("StdioServer(%q) returned unexpected server instance", tc.val)
		}
	}
}

func TestHTTPAccessLevelsAndTokens(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("MCP_TOKEN sees all tools while MCP_READ_TOKEN sees read-only tools", func(t *testing.T) {
		vars := map[string]string{
			EnvProjects:  dummyProjectsJSON,
			EnvToken:     "dummy-full-token",
			EnvReadToken: "dummy-read-token",
		}
		ts := httptest.NewServer(Handler(func(k string) string { return vars[k] }))
		defer ts.Close()

		fullTools := listHTTPClientTools(t, ctx, ts.URL+"/mcp", "dummy-full-token")
		if len(fullTools) != expectedFullToolCount {
			t.Fatalf("full token tool count = %d, want %d", len(fullTools), expectedFullToolCount)
		}

		readTools := listHTTPClientTools(t, ctx, ts.URL+"/mcp", "dummy-read-token")
		if len(readTools) != expectedReadToolCount {
			t.Fatalf("read token tool count = %d, want %d", len(readTools), expectedReadToolCount)
		}

		writeNames := allWriteToolNames()
		for _, tool := range readTools {
			if slices.Contains(writeNames, tool.Name) {
				t.Errorf("read-only server exposed write tool %q", tool.Name)
			}
			if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
				t.Errorf("read-only tool %q missing ReadOnlyHint=true", tool.Name)
			}
		}
	})

	t.Run("MCP_READ_ONLY=1 caps MCP_TOKEN to read-only tools", func(t *testing.T) {
		vars := map[string]string{
			EnvProjects: dummyProjectsJSON,
			EnvToken:    "dummy-full-token",
			EnvReadOnly: "1",
		}
		ts := httptest.NewServer(Handler(func(k string) string { return vars[k] }))
		defer ts.Close()

		tools := listHTTPClientTools(t, ctx, ts.URL+"/mcp", "dummy-full-token")
		if len(tools) != expectedReadToolCount {
			t.Fatalf("MCP_READ_ONLY=1 tool count = %d, want %d", len(tools), expectedReadToolCount)
		}
	})

	t.Run("only MCP_READ_TOKEN set works without MCP_TOKEN", func(t *testing.T) {
		vars := map[string]string{
			EnvProjects:  dummyProjectsJSON,
			EnvReadToken: "dummy-only-read-token",
		}
		ts := httptest.NewServer(Handler(func(k string) string { return vars[k] }))
		defer ts.Close()

		tools := listHTTPClientTools(t, ctx, ts.URL+"/mcp", "dummy-only-read-token")
		if len(tools) != expectedReadToolCount {
			t.Fatalf("MCP_READ_TOKEN-only tool count = %d, want %d", len(tools), expectedReadToolCount)
		}
	})

	badEnvCases := []struct {
		name string
		vars map[string]string
	}{
		{
			name: "neither token set",
			vars: map[string]string{EnvProjects: dummyProjectsJSON},
		},
		{
			name: "identical full and read tokens",
			vars: map[string]string{
				EnvProjects:  dummyProjectsJSON,
				EnvToken:     "same-token",
				EnvReadToken: "same-token",
			},
		},
		{
			name: "invalid MCP_READ_ONLY value",
			vars: map[string]string{
				EnvProjects: dummyProjectsJSON,
				EnvToken:    "dummy-full-token",
				EnvReadOnly: "yes",
			},
		},
	}

	for _, tc := range badEnvCases {
		t.Run(tc.name, func(t *testing.T) {
			h := Handler(func(k string) string { return tc.vars[k] })
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
			req.Header.Set("Authorization", "Bearer dummy-full-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", rec.Code)
			}
		})
	}
}

func TestReadOnlyServerCannotCallAnyWriteTool(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected upstream HTTP call from read-only server: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer upstream.Close()

	env, err := NewEnv(dummyProjectsJSON, upstream.Client(), revenuecat.WithBaseURL(upstream.URL))
	if err != nil {
		t.Fatalf("NewEnv error: %v", err)
	}

	ctx := WithEnv(context.Background(), env)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := NewReadOnly().Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	defer session.Close()

	writeTools := allWriteToolNames()
	if len(writeTools) != expectedWriteToolCount {
		t.Fatalf("len(writeTools) = %d, want %d", len(writeTools), expectedWriteToolCount)
	}

	for _, name := range writeTools {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: map[string]any{"project": "alpha"},
		})
		if err == nil {
			t.Errorf("read-only server allowed CallTool(%q), want unknown-tool error", name)
		}
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(clone)
}

func TestHTTPReadTokenRejectsWriteCallWhileFullTokenSucceeds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	vars := map[string]string{
		EnvProjects:  dummyProjectsJSON,
		EnvToken:     "dummy-full-token",
		EnvReadToken: "dummy-read-token",
	}
	ts := httptest.NewServer(Handler(func(k string) string { return vars[k] }))
	defer ts.Close()

	connect := func(token string) *mcp.ClientSession {
		httpClient := &http.Client{
			Transport: bearerTransport{token: token, base: http.DefaultTransport},
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "test-http", Version: "1.0"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint:   ts.URL + "/mcp",
			HTTPClient: httpClient,
		}, nil)
		if err != nil {
			t.Fatalf("Connect(%s): %v", token, err)
		}
		t.Cleanup(func() { _ = session.Close() })
		return session
	}

	readSession := connect("dummy-read-token")
	if _, err := readSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_product",
		Arguments: map[string]any{"project": "alpha", "product_id": "prod_dummy"},
	}); err == nil {
		t.Fatal("MCP_READ_TOKEN CallTool(delete_product) succeeded, want unknown-tool error")
	}

	fullSession := connect("dummy-full-token")
	res, err := fullSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "list_projects",
	})
	if err != nil || res.IsError {
		t.Fatalf("MCP_TOKEN CallTool(list_projects) = (%+v, %v), want success", res, err)
	}
	// Passing invalid input to a write tool proves the write tool IS registered on MCP_TOKEN
	// and runs input validation without making an outbound network call.
	writeRes, err := fullSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_product",
		Arguments: map[string]any{"project": "alpha", "product_id": ""},
	})
	if err != nil {
		t.Fatalf("MCP_TOKEN CallTool(delete_product) protocol error: %v", err)
	}
	if !writeRes.IsError || !strings.Contains(writeRes.Content[0].(*mcp.TextContent).Text, "product_id is required") {
		t.Fatalf("unexpected MCP_TOKEN delete_product validation response: %+v", writeRes)
	}
}

func listHTTPClientTools(t *testing.T, ctx context.Context, endpoint, token string) []*mcp.Tool {
	t.Helper()

	httpClient := &http.Client{
		Transport: bearerTransport{token: token, base: http.DefaultTransport},
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-http", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		t.Fatalf("MCP HTTP Connect failed: %v", err)
	}
	defer session.Close()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	return res.Tools
}
