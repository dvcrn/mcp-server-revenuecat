package server

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAllWriteToolsRoutingAndPayloads(t *testing.T) {
	t.Parallel()

	var (
		mu       sync.Mutex
		recorded []recordedRequest
	)

	session, ctx := newTestSession(t, New(), func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		recorded = append(recorded, recordedRequest{
			Method:      r.Method,
			RawPath:     r.URL.EscapedPath(),
			Path:        r.URL.Path,
			Auth:        r.Header.Get("Authorization"),
			ContentType: r.Header.Get("Content-Type"),
			Body:        body,
		})
		mu.Unlock()

		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"ok"}`))
	})

	cases := []struct {
		name        string
		tool        string
		args        map[string]any
		wantMethod  string
		wantAuth    string
		wantEscPath string
		wantBody    string // empty means bodyless request
	}{
		{
			name: "create_customer",
			tool: "create_customer",
			args: map[string]any{
				"project":    "ios_prod",
				"id":         "cust_dummy_new",
				"attributes": []map[string]any{{"name": "$email", "value": "user@example.com"}},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers",
			wantBody:    `{"attributes":[{"name":"$email","value":"user@example.com"}],"id":"cust_dummy_new"}`,
		},
		{
			name:        "delete_customer",
			tool:        "delete_customer",
			args:        map[string]any{"project": "ios_prod", "customer_id": "cust/dummy#1"},
			wantMethod:  http.MethodDelete,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust%2Fdummy%231",
		},
		{
			name: "set_customer_attributes set and delete",
			tool: "set_customer_attributes",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"attributes": []map[string]any{
					{"name": "$displayName", "value": "Jane Example"},
					{"name": "legacy_tier", "delete": true},
				},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/attributes",
			wantBody:    `{"attributes":[{"name":"$displayName","value":"Jane Example"},{"name":"legacy_tier","value":null}]}`,
		},
		{
			name: "grant_customer_entitlement",
			tool: "grant_customer_entitlement",
			args: map[string]any{
				"project":        "ios_prod",
				"customer_id":    "cust_dummy_1",
				"entitlement_id": "entl_dummy_pro",
				"expires_at":     int64(1900000000000),
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/actions/grant_entitlement",
			wantBody:    `{"entitlement_id":"entl_dummy_pro","expires_at":1900000000000}`,
		},
		{
			name: "revoke_customer_entitlement",
			tool: "revoke_customer_entitlement",
			args: map[string]any{
				"project":        "ios_prod",
				"customer_id":    "cust_dummy_1",
				"entitlement_id": "entl_dummy_pro",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/actions/revoke_granted_entitlement",
			wantBody:    `{"entitlement_id":"entl_dummy_pro"}`,
		},
		{
			name: "assign_customer_offering set",
			tool: "assign_customer_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"offering_id": "ofrng_dummy_promo",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/actions/assign_offering",
			wantBody:    `{"offering_id":"ofrng_dummy_promo"}`,
		},
		{
			name: "assign_customer_offering clear sends explicit null",
			tool: "assign_customer_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"clear":       true,
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/customers/cust_dummy_1/actions/assign_offering",
			wantBody:    `{"offering_id":null}`,
		},
		{
			name: "transfer_customer",
			tool: "transfer_customer",
			args: map[string]any{
				"project":            "android_prod",
				"customer_id":        "cust_dummy_src",
				"target_customer_id": "cust_dummy_dst",
				"app_ids":            []string{"app_dummy_1"},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_android",
			wantEscPath: "/v2/projects/proj5ef6g7h8/customers/cust_dummy_src/actions/transfer",
			wantBody:    `{"app_ids":["app_dummy_1"],"target_customer_id":"cust_dummy_dst"}`,
		},
		{
			name:        "cancel_subscription",
			tool:        "cancel_subscription",
			args:        map[string]any{"project": "ios_prod", "subscription_id": "sub_dummy_1"},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/subscriptions/sub_dummy_1/actions/cancel",
		},
		{
			name:        "refund_subscription",
			tool:        "refund_subscription",
			args:        map[string]any{"project": "ios_prod", "subscription_id": "sub_dummy_1"},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/subscriptions/sub_dummy_1/actions/refund",
		},
		{
			name: "extend_subscription by days",
			tool: "extend_subscription",
			args: map[string]any{
				"project":            "ios_prod",
				"subscription_id":    "sub_dummy_1",
				"extend_by_days":     7,
				"extend_reason_code": "customer_satisfaction",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/subscriptions/sub_dummy_1/actions/extend",
			wantBody:    `{"extend_by_days":7,"extend_reason_code":"customer_satisfaction"}`,
		},
		{
			name:        "refund_purchase",
			tool:        "refund_purchase",
			args:        map[string]any{"project": "ios_prod", "purchase_id": "pur_dummy_1"},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/purchases/pur_dummy_1/actions/refund",
		},
		{
			name: "create_product",
			tool: "create_product",
			args: map[string]any{
				"project":               "ios_prod",
				"store_identifier":      "com.example.pro.monthly",
				"app_id":                "app_dummy_1",
				"type":                  "subscription",
				"display_name":          "Pro Monthly",
				"subscription_duration": "P1M",
				"title":                 "Pro Monthly Plan",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/products",
			wantBody:    `{"app_id":"app_dummy_1","display_name":"Pro Monthly","store_identifier":"com.example.pro.monthly","subscription":{"duration":"P1M"},"title":"Pro Monthly Plan","type":"subscription"}`,
		},
		{
			name:        "delete_product",
			tool:        "delete_product",
			args:        map[string]any{"project": "ios_prod", "product_id": "prod_dummy_1"},
			wantMethod:  http.MethodDelete,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/products/prod_dummy_1",
		},
		{
			name: "create_entitlement",
			tool: "create_entitlement",
			args: map[string]any{
				"project":      "ios_prod",
				"lookup_key":   "pro",
				"display_name": "Pro Access",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements",
			wantBody:    `{"display_name":"Pro Access","lookup_key":"pro"}`,
		},
		{
			name: "update_entitlement",
			tool: "update_entitlement",
			args: map[string]any{
				"project":        "ios_prod",
				"entitlement_id": "entl_dummy_1",
				"display_name":   "Pro Plus Access",
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1",
			wantBody:    `{"display_name":"Pro Plus Access"}`,
		},
		{
			name:        "delete_entitlement",
			tool:        "delete_entitlement",
			args:        map[string]any{"project": "ios_prod", "entitlement_id": "entl_dummy_1"},
			wantMethod:  http.MethodDelete,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1",
		},
		{
			name: "attach_entitlement_products",
			tool: "attach_entitlement_products",
			args: map[string]any{
				"project":        "ios_prod",
				"entitlement_id": "entl_dummy_1",
				"product_ids":    []string{"prod_dummy_1", "prod_dummy_2"},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1/actions/attach_products",
			wantBody:    `{"product_ids":["prod_dummy_1","prod_dummy_2"]}`,
		},
		{
			name: "detach_entitlement_products",
			tool: "detach_entitlement_products",
			args: map[string]any{
				"project":        "ios_prod",
				"entitlement_id": "entl_dummy_1",
				"product_ids":    []string{"prod_dummy_2"},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/entitlements/entl_dummy_1/actions/detach_products",
			wantBody:    `{"product_ids":["prod_dummy_2"]}`,
		},
		{
			name: "create_offering",
			tool: "create_offering",
			args: map[string]any{
				"project":      "ios_prod",
				"lookup_key":   "default",
				"display_name": "Default Offering",
				"metadata":     map[string]any{"banner": "spring"},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings",
			wantBody:    `{"display_name":"Default Offering","lookup_key":"default","metadata":{"banner":"spring"}}`,
		},
		{
			name: "update_offering with clear_metadata",
			tool: "update_offering",
			args: map[string]any{
				"project":        "ios_prod",
				"offering_id":    "ofrng_dummy_1",
				"is_current":     true,
				"clear_metadata": true,
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1",
			wantBody:    `{"is_current":true,"metadata":null}`,
		},
		{
			name: "update_offering with empty metadata object",
			tool: "update_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"offering_id": "ofrng_dummy_1",
				"metadata":    map[string]any{},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1",
			wantBody:    `{"metadata":{}}`,
		},
		{
			name:        "delete_offering",
			tool:        "delete_offering",
			args:        map[string]any{"project": "ios_prod", "offering_id": "ofrng_dummy_1"},
			wantMethod:  http.MethodDelete,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1",
		},
		{
			name: "create_offering_package",
			tool: "create_offering_package",
			args: map[string]any{
				"project":      "ios_prod",
				"offering_id":  "ofrng_dummy_1",
				"lookup_key":   "$rc_monthly",
				"display_name": "Monthly",
				"position":     1,
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/offerings/ofrng_dummy_1/packages",
			wantBody:    `{"display_name":"Monthly","lookup_key":"$rc_monthly","position":1}`,
		},
		{
			name: "update_package",
			tool: "update_package",
			args: map[string]any{
				"project":      "ios_prod",
				"package_id":   "pkge_dummy_1",
				"display_name": "Monthly Plan",
				"position":     2,
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1",
			wantBody:    `{"display_name":"Monthly Plan","position":2}`,
		},
		{
			name:        "delete_package",
			tool:        "delete_package",
			args:        map[string]any{"project": "ios_prod", "package_id": "pkge_dummy_1"},
			wantMethod:  http.MethodDelete,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1",
		},
		{
			name: "attach_package_products defaults eligibility_criteria to all",
			tool: "attach_package_products",
			args: map[string]any{
				"project":    "ios_prod",
				"package_id": "pkge_dummy_1",
				"products": []map[string]any{
					{"product_id": "prod_dummy_1"},
					{"product_id": "prod_dummy_2", "eligibility_criteria": "google_sdk_ge_6"},
				},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1/actions/attach_products",
			wantBody:    `{"products":[{"eligibility_criteria":"all","product_id":"prod_dummy_1"},{"eligibility_criteria":"google_sdk_ge_6","product_id":"prod_dummy_2"}]}`,
		},
		{
			name: "detach_package_products",
			tool: "detach_package_products",
			args: map[string]any{
				"project":     "ios_prod",
				"package_id":  "pkge_dummy_1",
				"product_ids": []string{"prod_dummy_1"},
			},
			wantMethod:  http.MethodPost,
			wantAuth:    "Bearer sk_dummy_ios",
			wantEscPath: "/v2/projects/proj1ab2c3d4/packages/pkge_dummy_1/actions/detach_products",
			wantBody:    `{"product_ids":["prod_dummy_1"]}`,
		},
	}

	seenTools := make(map[string]bool)
	for i, tc := range cases {
		seenTools[tc.tool] = true
		res, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      tc.tool,
			Arguments: tc.args,
		})
		if err != nil {
			t.Fatalf("%s CallTool error: %v", tc.name, err)
		}
		if res.IsError {
			t.Fatalf("%s returned tool error: %+v", tc.name, res.Content)
		}

		if tc.wantMethod == http.MethodDelete {
			text := res.Content[0].(*mcp.TextContent).Text
			if text != `{"ok":true}` {
				t.Errorf("%s empty 204 body result = %q, want {\"ok\":true}", tc.name, text)
			}
		}

		mu.Lock()
		got := recorded[i]
		mu.Unlock()

		if got.Method != tc.wantMethod {
			t.Errorf("%s method = %s, want %s", tc.name, got.Method, tc.wantMethod)
		}
		if got.Auth != tc.wantAuth {
			t.Errorf("%s Auth = %q, want %q", tc.name, got.Auth, tc.wantAuth)
		}
		if got.RawPath != tc.wantEscPath {
			t.Errorf("%s RawPath = %q, want %q", tc.name, got.RawPath, tc.wantEscPath)
		}
		if tc.wantBody == "" {
			if len(got.Body) != 0 {
				t.Errorf("%s sent unexpected body %q", tc.name, string(got.Body))
			}
			if got.ContentType != "" {
				t.Errorf("%s Content-Type = %q, want empty for bodyless request", tc.name, got.ContentType)
			}
		} else {
			if got.ContentType != "application/json" {
				t.Errorf("%s Content-Type = %q, want application/json", tc.name, got.ContentType)
			}
			if !bytes.Equal(got.Body, []byte(tc.wantBody)) {
				t.Errorf("%s body = %s, want %s", tc.name, string(got.Body), tc.wantBody)
			}
		}
	}

	if len(seenTools) != expectedWriteToolCount {
		t.Fatalf("tested %d distinct write tools, want %d", len(seenTools), expectedWriteToolCount)
	}
}

func TestWriteToolValidationErrors(t *testing.T) {
	t.Parallel()

	session, ctx := newTestSession(t, New(), func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid write arguments must not reach upstream: %s %s", r.Method, r.URL.Path)
	})

	errCases := []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{
			name: "grant_customer_entitlement rejects 10-digit Unix seconds",
			tool: "grant_customer_entitlement",
			args: map[string]any{
				"project":        "ios_prod",
				"customer_id":    "cust_dummy_1",
				"entitlement_id": "entl_dummy_pro",
				"expires_at":     int64(1900000000),
			},
			want: "milliseconds",
		},
		{
			name: "extend_subscription rejects seconds in extend_until_ms",
			tool: "extend_subscription",
			args: map[string]any{
				"project":         "ios_prod",
				"subscription_id": "sub_dummy_1",
				"extend_until_ms": int64(1900000000),
			},
			want: "milliseconds",
		},
		{
			name: "extend_subscription rejects both days and until_ms",
			tool: "extend_subscription",
			args: map[string]any{
				"project":         "ios_prod",
				"subscription_id": "sub_dummy_1",
				"extend_by_days":  7,
				"extend_until_ms": int64(1900000000000),
			},
			want: "pass exactly one of extend_by_days or extend_until_ms",
		},
		{
			name: "extend_subscription rejects neither days nor until_ms",
			tool: "extend_subscription",
			args: map[string]any{
				"project":         "ios_prod",
				"subscription_id": "sub_dummy_1",
			},
			want: "pass exactly one of extend_by_days or extend_until_ms",
		},
		{
			name: "assign_customer_offering rejects missing offering_id and clear=false",
			tool: "assign_customer_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
			},
			want: "pass offering_id to assign an offering, or clear: true",
		},
		{
			name: "assign_customer_offering rejects both offering_id and clear=true",
			tool: "assign_customer_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"offering_id": "ofrng_dummy_1",
				"clear":       true,
			},
			want: "not both",
		},
		{
			name: "set_customer_attributes rejects missing value without delete=true",
			tool: "set_customer_attributes",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"attributes":  []map[string]any{{"name": "$email"}},
			},
			want: "value is required unless delete is true",
		},
		{
			name: "set_customer_attributes rejects value together with delete=true",
			tool: "set_customer_attributes",
			args: map[string]any{
				"project":     "ios_prod",
				"customer_id": "cust_dummy_1",
				"attributes":  []map[string]any{{"name": "$email", "value": "user@example.com", "delete": true}},
			},
			want: "omit value when delete is true",
		},
		{
			name: "update_offering rejects empty patch",
			tool: "update_offering",
			args: map[string]any{
				"project":     "ios_prod",
				"offering_id": "ofrng_dummy_1",
			},
			want: "pass at least one field to update",
		},
		{
			name: "update_offering rejects metadata and clear_metadata together",
			tool: "update_offering",
			args: map[string]any{
				"project":        "ios_prod",
				"offering_id":    "ofrng_dummy_1",
				"metadata":       map[string]any{"k": "v"},
				"clear_metadata": true,
			},
			want: "not both",
		},
		{
			name: "update_package rejects empty patch",
			tool: "update_package",
			args: map[string]any{
				"project":    "ios_prod",
				"package_id": "pkge_dummy_1",
			},
			want: "pass at least one field to update",
		},
		{
			name: "update_offering rejects empty metadata and clear_metadata together",
			tool: "update_offering",
			args: map[string]any{
				"project":        "ios_prod",
				"offering_id":    "ofrng_dummy_1",
				"metadata":       map[string]any{},
				"clear_metadata": true,
			},
			want: "not both",
		},
		{
			name: "create_customer rejects dot-traversal id",
			tool: "create_customer",
			args: map[string]any{
				"project": "ios_prod",
				"id":      "..",
			},
			want: "id cannot be",
		},
		{
			name: "transfer_customer rejects same source and target",
			tool: "transfer_customer",
			args: map[string]any{
				"project":            "ios_prod",
				"customer_id":        "cust_dummy_1",
				"target_customer_id": "cust_dummy_1",
			},
			want: "must differ",
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
