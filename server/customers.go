package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type listCustomersInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	Search        string `json:"search,omitempty" jsonschema:"Search filter matching customer email ($email attribute), app user ID, store transaction ID, or Apple order ID."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getCustomerInput struct {
	Project    string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID string   `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID, such as $RCAnonymousID:... or user ID)."`
	Expand     []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: attributes."`
}

type customerSubresourceListInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID    string `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

func registerCustomerTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_customers",
		Description: "List or search customers in a RevenueCat project. Pass `search` to look up by email, app user ID, or store transaction ID.",
		Annotations: readOnly(),
	}, listCustomers)

	add(r, &mcp.Tool{
		Name:        "get_customer",
		Description: "Get a customer by `customer_id`, including active entitlements and experiment enrollment. Pass `expand: [\"attributes\"]` to include subscriber attributes.",
		Annotations: readOnly(),
	}, getCustomer)

	add(r, &mcp.Tool{
		Name:        "list_customer_active_entitlements",
		Description: "List active entitlements currently held by `customer_id`.",
		Annotations: readOnly(),
	}, listCustomerActiveEntitlements)

	add(r, &mcp.Tool{
		Name:        "list_customer_aliases",
		Description: "List aliases associated with `customer_id`.",
		Annotations: readOnly(),
	}, listCustomerAliases)

	add(r, &mcp.Tool{
		Name:        "list_customer_attributes",
		Description: "List custom and reserved attributes stored on `customer_id`.",
		Annotations: readOnly(),
	}, listCustomerAttributes)
}

func listCustomers(ctx context.Context, env *Env, in listCustomersInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}
	if err := q.SetString("search", in.Search); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/customers", []string{"projects", p.ProjectID, "customers"}, q)
}

func getCustomer(ctx context.Context, env *Env, in getCustomerInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	customerID, err := requireID("customer_id", in.CustomerID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/customers/{customer_id}", []string{"projects", p.ProjectID, "customers", customerID}, q)
}

func listCustomerActiveEntitlements(ctx context.Context, env *Env, in customerSubresourceListInput) (*mcp.CallToolResult, any, error) {
	return listCustomerSubresource(ctx, env, in, "active_entitlements")
}

func listCustomerAliases(ctx context.Context, env *Env, in customerSubresourceListInput) (*mcp.CallToolResult, any, error) {
	return listCustomerSubresource(ctx, env, in, "aliases")
}

func listCustomerAttributes(ctx context.Context, env *Env, in customerSubresourceListInput) (*mcp.CallToolResult, any, error) {
	return listCustomerSubresource(ctx, env, in, "attributes")
}

func listCustomerSubresource(ctx context.Context, env *Env, in customerSubresourceListInput, resource string) (*mcp.CallToolResult, any, error) {
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

	return getJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/customers/{customer_id}/"+resource,
		[]string{"projects", p.ProjectID, "customers", customerID, resource},
		q,
	)
}
