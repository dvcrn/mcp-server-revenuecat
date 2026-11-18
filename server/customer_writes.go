package server

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

	return postJSON(ctx, p, "/v2/projects/{project_id}/customers", []string{"projects", p.ProjectID, "customers"}, body)
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
		body,
	)
}
