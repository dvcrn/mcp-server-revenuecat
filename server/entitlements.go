package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type listEntitlementsInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: items.product."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getEntitlementInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	EntitlementID string   `json:"entitlement_id" jsonschema:"RevenueCat entitlement ID (entl...)."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: product."`
}

type listEntitlementProductsInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	EntitlementID string `json:"entitlement_id" jsonschema:"RevenueCat entitlement ID (entl...)."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type createEntitlementInput struct {
	Project     string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	LookupKey   string `json:"lookup_key" jsonschema:"Unique entitlement identifier checked by apps, such as pro."`
	DisplayName string `json:"display_name" jsonschema:"Human-readable entitlement display name."`
}

type updateEntitlementInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	EntitlementID string `json:"entitlement_id" jsonschema:"RevenueCat entitlement ID (entl...)."`
	DisplayName   string `json:"display_name" jsonschema:"Updated human-readable display name."`
}

type deleteEntitlementInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	EntitlementID string `json:"entitlement_id" jsonschema:"RevenueCat entitlement ID (entl...) to delete."`
}

type entitlementProductsMutationInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	EntitlementID string   `json:"entitlement_id" jsonschema:"RevenueCat entitlement ID (entl...)."`
	ProductIDs    []string `json:"product_ids" jsonschema:"Product IDs (prod...) to attach or detach."`
}

func registerEntitlementTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_entitlements",
		Description: "List entitlements in a RevenueCat project.",
		Annotations: readOnly(),
	}, listEntitlements)

	add(r, &mcp.Tool{
		Name:        "get_entitlement",
		Description: "Get a RevenueCat entitlement by `entitlement_id`.",
		Annotations: readOnly(),
	}, getEntitlement)

	add(r, &mcp.Tool{
		Name:        "list_entitlement_products",
		Description: "List products attached to `entitlement_id`.",
		Annotations: readOnly(),
	}, listEntitlementProducts)

	add(r, &mcp.Tool{
		Name:        "create_entitlement",
		Description: "Create an entitlement in a RevenueCat project with `lookup_key` and `display_name`.",
		Annotations: mutating(),
	}, createEntitlement)

	add(r, &mcp.Tool{
		Name:        "update_entitlement",
		Description: "Update an entitlement's `display_name` by `entitlement_id`.",
		Annotations: idempotent(),
	}, updateEntitlement)

	add(r, &mcp.Tool{
		Name:        "delete_entitlement",
		Description: "Delete an entitlement from a RevenueCat project by `entitlement_id`, affecting access checks that depend on it. Confirm with the user first.",
		Annotations: destructive(),
	}, deleteEntitlement)

	add(r, &mcp.Tool{
		Name:        "attach_entitlement_products",
		Description: "Attach `product_ids` to `entitlement_id`.",
		Annotations: mutating(),
	}, attachEntitlementProducts)

	add(r, &mcp.Tool{
		Name:        "detach_entitlement_products",
		Description: "Detach `product_ids` from `entitlement_id`, which can remove entitlement access for subscribers of those products. Confirm with the user first.",
		Annotations: destructive(),
	}, detachEntitlementProducts)
}

func listEntitlements(ctx context.Context, env *Env, in listEntitlementsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/entitlements", []string{"projects", p.ProjectID, "entitlements"}, q)
}

func getEntitlement(ctx context.Context, env *Env, in getEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireID("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/entitlements/{entitlement_id}", []string{"projects", p.ProjectID, "entitlements", entID}, q)
}

func listEntitlementProducts(ctx context.Context, env *Env, in listEntitlementProductsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireID("entitlement_id", in.EntitlementID)
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
		"/v2/projects/{project_id}/entitlements/{entitlement_id}/products",
		[]string{"projects", p.ProjectID, "entitlements", entID, "products"},
		q,
	)
}

func createEntitlement(ctx context.Context, env *Env, in createEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	lookupKey, err := requireCleanField("lookup_key", in.LookupKey)
	if err != nil {
		return nil, nil, err
	}

	displayName, err := requireCleanField("display_name", in.DisplayName)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/entitlements",
		[]string{"projects", p.ProjectID, "entitlements"},
		map[string]any{"lookup_key": lookupKey, "display_name": displayName},
	)
}

func updateEntitlement(ctx context.Context, env *Env, in updateEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireID("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	displayName, err := requireCleanField("display_name", in.DisplayName)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/entitlements/{entitlement_id}",
		[]string{"projects", p.ProjectID, "entitlements", entID},
		map[string]any{"display_name": displayName},
	)
}

func deleteEntitlement(ctx context.Context, env *Env, in deleteEntitlementInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireID("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	return deleteJSON(ctx, p, "/v2/projects/{project_id}/entitlements/{entitlement_id}", []string{"projects", p.ProjectID, "entitlements", entID})
}

func attachEntitlementProducts(ctx context.Context, env *Env, in entitlementProductsMutationInput) (*mcp.CallToolResult, any, error) {
	return mutateEntitlementProducts(ctx, env, in, "attach_products")
}

func detachEntitlementProducts(ctx context.Context, env *Env, in entitlementProductsMutationInput) (*mcp.CallToolResult, any, error) {
	return mutateEntitlementProducts(ctx, env, in, "detach_products")
}

func mutateEntitlementProducts(ctx context.Context, env *Env, in entitlementProductsMutationInput, action string) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	entID, err := requireID("entitlement_id", in.EntitlementID)
	if err != nil {
		return nil, nil, err
	}

	productIDs, err := requireIDList("product_ids", in.ProductIDs)
	if err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/entitlements/{entitlement_id}/actions/"+action,
		[]string{"projects", p.ProjectID, "entitlements", entID, "actions", action},
		map[string]any{"product_ids": productIDs},
	)
}
