package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat/revenuecat"
)

type listCustomerPurchasesInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID    string   `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	Environment   string   `json:"environment,omitempty" jsonschema:"Optional store environment filter: production or sandbox."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand (for example items.product)."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type searchPurchasesInput struct {
	Project                 string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	StorePurchaseIdentifier string   `json:"store_purchase_identifier" jsonschema:"Store one-time purchase transaction identifier."`
	Expand                  []string `json:"expand,omitempty" jsonschema:"Optional fields to expand."`
}

type getPurchaseInput struct {
	Project    string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PurchaseID string   `json:"purchase_id" jsonschema:"RevenueCat purchase ID (pur...)."`
	Expand     []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: redemption."`
}

type listPurchaseEntitlementsInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PurchaseID    string `json:"purchase_id" jsonschema:"RevenueCat purchase ID (pur...)."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

func registerPurchaseTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_customer_purchases",
		Description: "List non-subscription purchases belonging to `customer_id`, optionally filtered by `environment` (production or sandbox).",
		Annotations: readOnly(),
	}, listCustomerPurchases)

	add(r, &mcp.Tool{
		Name:        "search_purchases",
		Description: "Search non-subscription purchases in a RevenueCat project by `store_purchase_identifier`.",
		Annotations: readOnly(),
	}, searchPurchases)

	add(r, &mcp.Tool{
		Name:        "get_purchase",
		Description: "Get a non-subscription purchase by `purchase_id`.",
		Annotations: readOnly(),
	}, getPurchase)

	add(r, &mcp.Tool{
		Name:        "list_purchase_entitlements",
		Description: "List entitlements unlocked by `purchase_id`.",
		Annotations: readOnly(),
	}, listPurchaseEntitlements)
}

func listCustomerPurchases(ctx context.Context, env *Env, in listCustomerPurchasesInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}
	if err := q.SetString("environment", in.Environment); err != nil {
		return nil, nil, err
	}
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/purchases",
		[]string{"projects", p.ProjectID, "customers", customerID, "purchases"},
		q,
	)
}

func searchPurchases(ctx context.Context, env *Env, in searchPurchasesInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.SetRequiredString("store_purchase_identifier", in.StorePurchaseIdentifier); err != nil {
		return nil, nil, err
	}
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/purchases", []string{"projects", p.ProjectID, "purchases"}, q)
}

func getPurchase(ctx context.Context, env *Env, in getPurchaseInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	purID, err := requireID("purchase_id", in.PurchaseID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/purchases/{purchase_id}",
		[]string{"projects", p.ProjectID, "purchases", purID},
		q,
	)
}

func listPurchaseEntitlements(ctx context.Context, env *Env, in listPurchaseEntitlementsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	purID, err := requireID("purchase_id", in.PurchaseID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}

	return getJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/purchases/{purchase_id}/entitlements",
		[]string{"projects", p.ProjectID, "purchases", purID, "entitlements"},
		q,
	)
}
