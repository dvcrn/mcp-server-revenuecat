package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type recordedRequest struct {
	Method      string
	RawPath     string
	Path        string
	Query       url.Values
	Auth        string
	ContentType string
	Body        []byte
}

func newTestSession(t *testing.T, srv *mcp.Server, handler http.HandlerFunc) (*mcp.ClientSession, context.Context) {
	t.Helper()

	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)

	rawProjects := `[
		{"alias":"ios_prod","name":"iOS Production","project_id":"proj1ab2c3d4","api_key":"sk_dummy_ios"},
		{"alias":"android_prod","name":"Android Production","project_id":"proj5ef6g7h8","api_key":"sk_dummy_android"}
	]`

	env, err := NewEnv(rawProjects, upstream.Client(), revenuecat.WithBaseURL(upstream.URL))
	if err != nil {
		t.Fatalf("NewEnv: %v", err)
	}

	ctx := WithEnv(context.Background(), env)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session, ctx
}

func TestListProjectsNeverExposesSecretKeys(t *testing.T) {
	t.Parallel()

	session, ctx := newTestSession(t, NewReadOnly(), func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("list_projects must not call upstream API: %s %s", r.Method, r.URL.Path)
	})

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_projects"})
	if err != nil {
		t.Fatalf("CallTool(list_projects): %v", err)
	}
	if res.IsError {
		t.Fatalf("list_projects returned tool error: %+v", res.Content)
	}

	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(raw), "sk_dummy_") {
		t.Fatalf("list_projects leaked dummy API key: %s", string(raw))
	}
	if !strings.Contains(string(raw), "proj1ab2c3d4") || !strings.Contains(string(raw), "android_prod") {
		t.Fatalf("list_projects missing expected projects: %s", string(raw))
	}
}

func TestAllReadToolsRoutingAndQueryEncoding(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		recorded []recordedRequest
	)

	session, ctx := newTestSession(t, NewReadOnly(), func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		recorded = append(recorded, recordedRequest{
			Method:  r.Method,
			RawPath: r.URL.EscapedPath(),
			Path:    r.URL.Path,
			Query:   r.URL.Query(),
			Auth:    r.Header.Get("Authorization"),
		})
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","items":[]}`))
	})

	cases := []struct {
		tool        string
		args        map[string]any
		wantAuth    string
		wantEscPath string
		checkQuery  func(t *testing.T, q url.Values)
	}{
		{
			tool:        "list_apps",
			args:        map[string]any{"project": "ios_prod", "limit": 250, "starting_after": "/v2/projects/proj1ab2c3d4/apps?starting_after=app_cursor&limit=20"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/apps",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("limit") != "100" || q.Get("starting_after") != "app_cursor" {
					t.Errorf("unexpected query: %v", q)
				}
			},
		},
		{
			tool:        "get_app",
			args:        map[string]any{"project": "proj5ef6g7h8", "app_id": "app1a2b3c4d"},
			wantAuth:    "Bearer sk_dummy_android",
			wantEscPath: "/v2/projects/proj5ef6g7h8/apps/app1a2b3c4d",
		},
		{
			tool:        "get_overview_metrics",
			args:        map[string]any{"project": "ios_prod", "currency": "USD"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/metrics/overview",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("currency") != "USD" {
					t.Errorf("currency = %q, want USD", q.Get("currency"))
				}
			},
		},
		{
			tool:        "get_chart_options",
			args:        map[string]any{"project": "ios_prod", "chart_name": "revenue", "realtime": true},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/charts/revenue/options",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("realtime") != "true" {
					t.Errorf("realtime = %q, want true", q.Get("realtime"))
				}
			},
		},
		{
			tool: "get_chart_data",
			args: map[string]any{
				"project":            "ios_prod",
				"chart_name":         "mrr",
				"start_date":         "2026-01-01",
				"end_date":           "2026-01-31",
				"resolution":         "day",
				"currency":           "EUR",
				"segment":            "country",
				"filters":            []map[string]any{{"name": "country", "values": []string{"US", "DE"}}},
				"selectors":          map[string]string{"revenue_type": "proceeds"},
				"aggregate":          []string{"average", "total"},
				"realtime":           false,
				"expand_periods":     true,
				"limit_num_segments": 5,
			},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/charts/mrr",
			checkQuery: func(t *testing.T, q url.Values) {
				if !slices.Equal(q["aggregate"], []string{"average", "total"}) {
					t.Errorf("aggregate = %v, want [average total]", q["aggregate"])
				}
				if q.Get("filters") != `[{"name":"country","values":["US","DE"]}]` {
					t.Errorf("filters = %q", q.Get("filters"))
				}
				if q.Get("selectors") != `{"revenue_type":"proceeds"}` {
					t.Errorf("selectors = %q", q.Get("selectors"))
				}
				if q.Get("limit_num_segments") != "5" || q.Get("realtime") != "false" || q.Get("expand_periods") != "true" {
					t.Errorf("unexpected chart flags: %v", q)
				}
			},
		},
		{
			tool:        "list_customers",
			args:        map[string]any{"project": "ios_prod", "search": "user@example.com"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("search") != "user@example.com" {
					t.Errorf("search = %q, want user@example.com", q.Get("search"))
				}
			},
		},
		{
			tool:        "get_customer",
			args:        map[string]any{"project": "ios_prod", "customer_id": "org/user#1?x=2", "expand": []string{"attributes"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/org%2Fuser%231%3Fx=2",
			checkQuery: func(t *testing.T, q url.Values) {
				if !slices.Equal(q["expand"], []string{"attributes"}) {
					t.Errorf("expand = %v, want [attributes]", q["expand"])
				}
			},
		},
		{
			tool:        "list_customer_active_entitlements",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/active_entitlements",
		},
		{
			tool:        "list_customer_aliases",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/aliases",
		},
		{
			tool:        "list_customer_attributes",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/attributes",
		},
		{
			tool:        "list_customer_subscriptions",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust_dummy_1", "environment": "sandbox", "expand": []string{"items.product"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/subscriptions",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("environment") != "sandbox" || !slices.Equal(q["expand"], []string{"items.product"}) {
					t.Errorf("unexpected query: %v", q)
				}
			},
		},
		{
			tool:        "search_subscriptions",
			args:        map[string]any{"project": "android_prod", "store_subscription_identifier": "GPA.0000-0000", "include_scheduled": true},
			wantAuth:    "Bearer sk_dummy_android",
			wantEscPath: "/v2/projects/proj5ef6g7h8/subscriptions",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("store_subscription_identifier") != "GPA.0000-0000" || q.Get("include_scheduled") != "true" {
					t.Errorf("unexpected query: %v", q)
				}
			},
		},
		{
			tool:        "get_subscription",
			args:        map[string]any{"project": "ios_prod", "subscription_id": "sub_dummy_1", "expand": []string{"redemption"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/subscriptions/sub_dummy_1",
		},
		{
			tool:        "list_subscription_entitlements",
			args:        map[string]any{"project": "ios_prod", "subscription_id": "sub_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/subscriptions/sub_dummy_1/entitlements",
		},
		{
			tool:        "list_customer_purchases",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust_dummy_1", "environment": "production"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/purchases",
		},
		{
			tool:        "search_purchases",
			args:        map[string]any{"project": "ios_prod", "store_purchase_identifier": "txn_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/purchases",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("store_purchase_identifier") != "txn_dummy_1" {
					t.Errorf("store_purchase_identifier = %q", q.Get("store_purchase_identifier"))
				}
			},
		},
		{
			tool:        "get_purchase",
			args:        map[string]any{"project": "ios_prod", "purchase_id": "pur_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/purchases/pur_dummy_1",
		},
		{
			tool:        "list_purchase_entitlements",
			args:        map[string]any{"project": "ios_prod", "purchase_id": "pur_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/purchases/pur_dummy_1/entitlements",
		},
		{
			tool:        "list_products",
			args:        map[string]any{"project": "ios_prod", "app_id": "app_dummy_1", "expand": []string{"items.app", "items.indicative_price"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/products",
			checkQuery: func(t *testing.T, q url.Values) {
				if q.Get("app_id") != "app_dummy_1" || !slices.Equal(q["expand"], []string{"items.app", "items.indicative_price"}) {
					t.Errorf("unexpected query: %v", q)
				}
			},
		},
		{
			tool:        "get_product",
			args:        map[string]any{"project": "ios_prod", "product_id": "prod_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/products/prod_dummy_1",
		},
		{
			tool:        "list_entitlements",
			args:        map[string]any{"project": "ios_prod", "expand": []string{"items.product"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements",
		},
		{
			tool:        "get_entitlement",
			args:        map[string]any{"project": "ios_prod", "entitlement_id": "entl_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1",
		},
		{
			tool:        "list_entitlement_products",
			args:        map[string]any{"project": "ios_prod", "entitlement_id": "entl_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1/products",
		},
		{
			tool:        "list_offerings",
			args:        map[string]any{"project": "ios_prod", "expand": []string{"items.package", "items.package.product"}},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings",
			checkQuery: func(t *testing.T, q url.Values) {
				if !slices.Equal(q["expand"], []string{"items.package", "items.package.product"}) {
					t.Errorf("expand = %v", q["expand"])
				}
			},
		},
		{
			tool:        "get_offering",
			args:        map[string]any{"project": "ios_prod", "offering_id": "ofrng_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1",
		},
		{
			tool:        "list_offering_packages",
			args:        map[string]any{"project": "ios_prod", "offering_id": "ofrng_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1/packages",
		},
		{
			tool:        "get_package",
			args:        map[string]any{"project": "ios_prod", "package_id": "pkge_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1",
		},
		{
			tool:        "list_package_products",
			args:        map[string]any{"project": "ios_prod", "package_id": "pkge_dummy_1"},
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1/products",
		},
	}

	// 28 upstream GET tools + list_projects = 29 read tools.
	if len(cases)+1 != expectedReadToolCount {
		t.Fatalf("covered %d read tools (+ list_projects), want %d", len(cases)+1, expectedReadToolCount)
	}

	for i, tc := range cases {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      tc.tool,
			Arguments: tc.args,
		})
		if err != nil {
			t.Fatalf("%s CallTool error: %v", tc.tool, err)
		}
		if res.IsError {
			t.Fatalf("%s returned tool error: %+v", tc.tool, res.Content)
		}

		mu.Lock()
		got := recorded[i]
		mu.Unlock()

		if got.Method != http.MethodGet {
			t.Errorf("%s method = %s, want GET", tc.tool, got.Method)
		}
		if got.Auth != tc.wantAuth {
			t.Errorf("%s Authorization = %q, want %q", tc.tool, got.Auth, tc.wantAuth)
		}
		if got.RawPath != tc.wantEscPath {
			t.Errorf("%s RawPath = %q, want %q", tc.tool, got.RawPath, tc.wantEscPath)
		}
		if tc.checkQuery != nil {
			tc.checkQuery(t, got.Query)
		}
	}
}

func TestReadToolInputValidation(t *testing.T) {
	t.Parallel()

	session, ctx := newTestSession(t, NewReadOnly(), func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("validation failure must not hit upstream: %s %s", r.Method, r.URL.Path)
	})

	errCases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "unknown project",
			tool: "list_apps",
			args: map[string]any{"project": "proj_unconfigured"},
			want: "unknown project",
		},
		{
			name: "limit zero",
			tool: "list_apps",
			args: map[string]any{"project": "ios_prod", "limit": 0},
			want: "limit must be at least 1",
		},
		{
			name: "next_page URL without cursor",
			tool: "list_apps",
			args: map[string]any{"project": "ios_prod", "starting_after": "/v2/projects/proj1ab2c3d4/apps?limit=20"},
			want: "has no starting_after query parameter",
		},
		{
			name: "dot traversal in customer_id",
			tool: "get_customer",
			args: map[string]any{"project": "ios_prod", "customer_id": ".."},
			want: "invalid path segment",
		},
		{
			name: "empty filter values",
			tool: "get_chart_data",
			args: map[string]any{
				"project":    "ios_prod",
				"chart_name": "revenue",
				"filters":    []map[string]any{{"name": "country", "values": []string{}}},
			},
			want: "cannot be empty",
		},
	}

	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      tc.tool,
				Arguments: tc.args,
			})
			if err != nil {
				if !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("error = %v, want substring %q", err, tc.want)
				}
				return
			}
			if !res.IsError {
				t.Fatalf("expected tool error containing %q, got success", tc.want)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, tc.want) {
				t.Errorf("tool error = %q, want substring %q", text, tc.want)
			}
		})
	}
}
