package server

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

// minEpochMillis is 2001-09-09T01:46:40Z in Unix milliseconds, used to reject
// 10-digit Unix-second timestamps accidentally passed to millisecond fields.
const minEpochMillis int64 = 1_000_000_000_000

func registerWriteTools(r registry) {
	registerCustomerWriteTools(r)
	registerCatalogWriteTools(r)
}

func postJSON(ctx context.Context, p *Project, endpoint string, segments []string, q *revenuecat.Query, body any) (*mcp.CallToolResult, any, error) {
	var vals url.Values
	if q != nil {
		vals = q.Values()
	}

	resp, err := p.Client.Post(ctx, endpoint, segments, vals, body)
	if err != nil {
		return nil, nil, err
	}

	return jsonBodyResult(resp), nil, nil
}

func deleteJSON(ctx context.Context, p *Project, endpoint string, segments []string) (*mcp.CallToolResult, any, error) {
	resp, err := p.Client.Delete(ctx, endpoint, segments)
	if err != nil {
		return nil, nil, err
	}

	return jsonBodyResult(resp), nil, nil
}

func requireEpochMillis(name string, ms int64) error {
	if ms < minEpochMillis {
		return fmt.Errorf("%s must be a Unix timestamp in milliseconds (>= %d)", name, minEpochMillis)
	}

	return nil
}

func hasControlOrInvalidUTF8(s string) bool {
	if !utf8.ValidString(s) {
		return true
	}

	for _, r := range s {
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) {
			return true
		}
	}

	return false
}

func requireCleanField(name, val string) (string, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if hasControlOrInvalidUTF8(trimmed) {
		return "", fmt.Errorf("%s contains invalid characters", name)
	}

	return trimmed, nil
}

func requireIDList(name string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s must contain at least one ID", name)
	}

	out := make([]string, len(ids))
	for i, raw := range ids {
		cleaned, err := requireCleanField(name, raw)
		if err != nil {
			return nil, err
		}
		out[i] = cleaned
	}

	return out, nil
}

func requireChange(body map[string]any, fields ...string) error {
	if len(body) == 0 {
		return fmt.Errorf("pass at least one field to update (%s)", strings.Join(fields, ", "))
	}

	return nil
}

type customerAttributeValue struct {
	Name  string `json:"name" jsonschema:"Attribute name, such as $email, $displayName, or a custom attribute key."`
	Value string `json:"value" jsonschema:"Non-empty attribute string value."`
}

type createCustomerInput struct {
	Project    string                   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ID         string                   `json:"id" jsonschema:"App User ID for the new customer (ASCII letters, digits, underscores, and hyphens: [0-9a-zA-Z_-]+)."`
	Attributes []customerAttributeValue `json:"attributes,omitempty" jsonschema:"Optional initial customer attributes to set on creation."`
}

type deleteCustomerInput struct {
	Project    string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID string `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID) to delete."`
}

type customerAttributePatch struct {
	Name   string  `json:"name" jsonschema:"Attribute name, such as $email, $displayName, or a custom attribute key."`
	Value  *string `json:"value,omitempty" jsonschema:"Non-empty attribute value to set. Omit when delete is true."`
	Delete bool    `json:"delete,omitempty" jsonschema:"Whether to delete this attribute (omit value when true)."`
}

type setCustomerAttributesInput struct {
	Project    string                   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID string                   `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	Attributes []customerAttributePatch `json:"attributes" jsonschema:"Customer attributes to set or delete."`
}

type grantCustomerEntitlementInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID    string `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	EntitlementID string `json:"entitlement_id" jsonschema:"Entitlement ID (entl...) to grant."`
	ExpiresAt     int64  `json:"expires_at" jsonschema:"Grant expiration timestamp in Unix milliseconds."`
}

type revokeCustomerEntitlementInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID    string `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	EntitlementID string `json:"entitlement_id" jsonschema:"Entitlement ID (entl...) whose promotional grant should be revoked."`
}

type assignCustomerOfferingInput struct {
	Project    string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID string `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	OfferingID string `json:"offering_id,omitempty" jsonschema:"Offering ID (ofrng...) to assign to this customer. Omit when clear is true."`
	Clear      bool   `json:"clear,omitempty" jsonschema:"Set true to clear any offering override on this customer. Omit offering_id when clear is true."`
}

type transferCustomerInput struct {
	Project          string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID       string   `json:"customer_id" jsonschema:"Source customer ID whose subscriptions and purchases will be transferred."`
	TargetCustomerID string   `json:"target_customer_id" jsonschema:"Destination customer ID receiving the transferred subscriptions and purchases."`
	AppIDs           []string `json:"app_ids,omitempty" jsonschema:"Optional app IDs restricting the transfer to specific apps."`
}

type subscriptionActionInput struct {
	Project        string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	SubscriptionID string `json:"subscription_id" jsonschema:"RevenueCat subscription ID (sub...)."`
}

type extendSubscriptionInput struct {
	Project          string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	SubscriptionID   string `json:"subscription_id" jsonschema:"RevenueCat subscription ID (sub...)."`
	ExtendByDays     *int   `json:"extend_by_days,omitempty" jsonschema:"Number of days to extend the subscription (at least 1, max 90 on Apple App Store). Mutually exclusive with extend_until_ms."`
	ExtendUntilMs    *int64 `json:"extend_until_ms,omitempty" jsonschema:"Target expiration timestamp in Unix milliseconds. Mutually exclusive with extend_by_days."`
	ExtendReasonCode string `json:"extend_reason_code,omitempty" jsonschema:"Optional extension reason code: undeclared, customer_satisfaction, other, or service_issue_or_outage (required for Apple App Store subscriptions)."`
}

type refundPurchaseInput struct {
	Project    string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PurchaseID string `json:"purchase_id" jsonschema:"RevenueCat purchase ID (pur...)."`
}

func registerCustomerWriteTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "create_customer",
		Description: "Create a customer in a RevenueCat project with `id` and optional initial `attributes`.",
		Annotations: mutating(),
	}, createCustomer)

	add(r, &mcp.Tool{
		Name:        "delete_customer",
		Description: "Permanently delete a customer and their subscriber history by `customer_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, deleteCustomer)

	add(r, &mcp.Tool{
		Name:        "set_customer_attributes",
		Description: "Set or delete subscriber attributes on `customer_id`. Pass `delete: true` without `value` to remove an attribute.",
		Annotations: idempotent(),
	}, setCustomerAttributes)

	add(r, &mcp.Tool{
		Name:        "grant_customer_entitlement",
		Description: "Grant a promotional entitlement (`entitlement_id`) to `customer_id` until `expires_at` (Unix milliseconds).",
		Annotations: mutating(),
	}, grantCustomerEntitlement)

	add(r, &mcp.Tool{
		Name:        "revoke_customer_entitlement",
		Description: "Revoke an active promotional entitlement grant (`entitlement_id`) on `customer_id` immediately. Confirm with the user first.",
		Annotations: destructive(),
	}, revokeCustomerEntitlement)

	add(r, &mcp.Tool{
		Name:        "assign_customer_offering",
		Description: "Assign an `offering_id` override to `customer_id`, or pass `clear: true` to remove an existing offering override.",
		Annotations: idempotent(),
	}, assignCustomerOffering)

	add(r, &mcp.Tool{
		Name:        "transfer_customer",
		Description: "Transfer subscriptions and one-time purchases from `customer_id` to `target_customer_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, transferCustomer)

	add(r, &mcp.Tool{
		Name:        "cancel_subscription",
		Description: "Cancel an active RevenueCat Web Billing (`rc_billing`) subscription by `subscription_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, cancelSubscription)

	add(r, &mcp.Tool{
		Name:        "refund_subscription",
		Description: "Refund the most recent payment of a RevenueCat Web Billing (`rc_billing`) subscription and revoke access immediately. Confirm with the user first.",
		Annotations: destructive(),
	}, refundSubscription)

	add(r, &mcp.Tool{
		Name:        "extend_subscription",
		Description: "Extend a subscription's renewal date by `extend_by_days` or until `extend_until_ms` (Unix milliseconds).",
		Annotations: mutating(),
	}, extendSubscription)

	add(r, &mcp.Tool{
		Name:        "refund_purchase",
		Description: "Refund a RevenueCat Web Billing (`rc_billing`) one-time purchase and revoke entitlement access immediately. Confirm with the user first.",
		Annotations: destructive(),
	}, refundPurchase)
}

func isValidNewCustomerID(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return len(s) > 0
}

func createCustomer(ctx context.Context, env *Env, in createCustomerInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireCleanField("id", in.ID)
	if err != nil {
		return nil, nil, err
	}
	if !isValidNewCustomerID(customerID) {
		return nil, nil, fmt.Errorf("id must contain only ASCII letters, digits, underscores, and hyphens ([0-9a-zA-Z_-]+)")
	}

	body := map[string]any{"id": customerID}
	if len(in.Attributes) > 0 {
		attrs := make([]map[string]any, len(in.Attributes))
		for i, attr := range in.Attributes {
			name, err := requireCleanField("attributes.name", attr.Name)
			if err != nil {
				return nil, nil, err
			}
			if attr.Value == "" {
				return nil, nil, fmt.Errorf("attributes[%q]: value is required", name)
			}
			if !utf8.ValidString(attr.Value) {
				return nil, nil, fmt.Errorf("attributes[%q]: value is not valid UTF-8", name)
			}
			attrs[i] = map[string]any{"name": name, "value": attr.Value}
		}
		body["attributes"] = attrs
	}

	return postJSON(ctx, p, "/v2/projects/{project_id}/customers", []string{"projects", p.ProjectID, "customers"}, nil, body)
}

func deleteCustomer(ctx context.Context, env *Env, in deleteCustomerInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	return deleteJSON(ctx, p, "/v2/projects/{project_id}/customers/{customer_id}", []string{"projects", p.ProjectID, "customers", customerID})
}

func setCustomerAttributes(ctx context.Context, env *Env, in setCustomerAttributesInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	if len(in.Attributes) == 0 {
		return nil, nil, fmt.Errorf("attributes must contain at least one item")
	}

	attrs := make([]map[string]any, len(in.Attributes))
	for i, item := range in.Attributes {
		name, err := requireCleanField("attributes.name", item.Name)
		if err != nil {
			return nil, nil, err
		}

		if item.Delete {
			if item.Value != nil {
				return nil, nil, fmt.Errorf("attributes[%q]: omit value when delete is true", name)
			}
			attrs[i] = map[string]any{"name": name, "value": nil}
			continue
		}

		if item.Value == nil || *item.Value == "" {
			return nil, nil, fmt.Errorf("attributes[%q]: value is required unless delete is true", name)
		}
		if !utf8.ValidString(*item.Value) {
			return nil, nil, fmt.Errorf("attributes[%q]: value is not valid UTF-8", name)
		}
		attrs[i] = map[string]any{"name": name, "value": *item.Value}
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/attributes",
		[]string{"projects", p.ProjectID, "customers", customerID, "attributes"},
		nil,
		map[string]any{"attributes": attrs},
	)
}

func grantCustomerEntitlement(ctx context.Context, env *Env, in grantCustomerEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireCleanField("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	if err := requireEpochMillis("expires_at", in.ExpiresAt); err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/actions/grant_entitlement",
		[]string{"projects", p.ProjectID, "customers", customerID, "actions", "grant_entitlement"},
		nil,
		map[string]any{"entitlement_id": entID, "expires_at": in.ExpiresAt},
	)
}

func revokeCustomerEntitlement(ctx context.Context, env *Env, in revokeCustomerEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireCleanField("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/actions/revoke_granted_entitlement",
		[]string{"projects", p.ProjectID, "customers", customerID, "actions", "revoke_granted_entitlement"},
		nil,
		map[string]any{"entitlement_id": entID},
	)
}

func assignCustomerOffering(ctx context.Context, env *Env, in assignCustomerOfferingInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	rawOffering := strings.TrimSpace(in.OfferingID)
	if in.Clear && rawOffering != "" {
		return nil, nil, fmt.Errorf("pass either offering_id or clear: true, not both")
	}
	if !in.Clear && rawOffering == "" {
		return nil, nil, fmt.Errorf("pass offering_id to assign an offering, or clear: true to remove an override")
	}

	var offeringVal any
	if !in.Clear {
		cleaned, err := requireCleanField("offering_id", rawOffering)
		if err != nil {
			return nil, nil, err
		}
		offeringVal = cleaned
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/actions/assign_offering",
		[]string{"projects", p.ProjectID, "customers", customerID, "actions", "assign_offering"},
		nil,
		map[string]any{"offering_id": offeringVal},
	)
}

func transferCustomer(ctx context.Context, env *Env, in transferCustomerInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	targetID, err := requireCleanField("target_customer_id", in.TargetCustomerID)
	if err != nil {
		return nil, nil, err
	}
	if targetID == "." || targetID == ".." {
		return nil, nil, fmt.Errorf("target_customer_id cannot be %q", targetID)
	}
	if customerID == targetID {
		return nil, nil, fmt.Errorf("target_customer_id must differ from customer_id")
	}

	body := map[string]any{"target_customer_id": targetID}
	if len(in.AppIDs) > 0 {
		appIDs, err := requireIDList("app_ids", in.AppIDs)
		if err != nil {
			return nil, nil, err
		}
		body["app_ids"] = appIDs
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/actions/transfer",
		[]string{"projects", p.ProjectID, "customers", customerID, "actions", "transfer"},
		nil,
		body,
	)
}

func cancelSubscription(ctx context.Context, env *Env, in subscriptionActionInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	subID, err := requireID("subscription_id", in.SubscriptionID)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/subscriptions/{subscription_id}/actions/cancel",
		[]string{"projects", p.ProjectID, "subscriptions", subID, "actions", "cancel"},
		nil,
		nil,
	)
}

func refundSubscription(ctx context.Context, env *Env, in subscriptionActionInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	subID, err := requireID("subscription_id", in.SubscriptionID)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/subscriptions/{subscription_id}/actions/refund",
		[]string{"projects", p.ProjectID, "subscriptions", subID, "actions", "refund"},
		nil,
		nil,
	)
}

func extendSubscription(ctx context.Context, env *Env, in extendSubscriptionInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	subID, err := requireID("subscription_id", in.SubscriptionID)
	if err != nil {
		return nil, nil, err
	}

	if (in.ExtendByDays == nil) == (in.ExtendUntilMs == nil) {
		return nil, nil, fmt.Errorf("pass exactly one of extend_by_days or extend_until_ms")
	}

	body := make(map[string]any, 2)
	if in.ExtendByDays != nil {
		if *in.ExtendByDays < 1 {
			return nil, nil, fmt.Errorf("extend_by_days must be at least 1")
		}
		body["extend_by_days"] = *in.ExtendByDays
	} else {
		if err := requireEpochMillis("extend_until_ms", *in.ExtendUntilMs); err != nil {
			return nil, nil, err
		}
		body["extend_until_ms"] = *in.ExtendUntilMs
	}

	if reason := strings.TrimSpace(in.ExtendReasonCode); reason != "" {
		switch reason {
		case "undeclared", "customer_satisfaction", "other", "service_issue_or_outage":
			body["extend_reason_code"] = reason
		default:
			return nil, nil, fmt.Errorf("extend_reason_code must be one of undeclared, customer_satisfaction, other, or service_issue_or_outage")
		}
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/subscriptions/{subscription_id}/actions/extend",
		[]string{"projects", p.ProjectID, "subscriptions", subID, "actions", "extend"},
		nil,
		body,
	)
}

func refundPurchase(ctx context.Context, env *Env, in refundPurchaseInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	purchaseID, err := requireID("purchase_id", in.PurchaseID)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/purchases/{purchase_id}/actions/refund",
		[]string{"projects", p.ProjectID, "purchases", purchaseID, "actions", "refund"},
		nil,
		nil,
	)
}
