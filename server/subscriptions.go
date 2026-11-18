package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type listCustomerSubscriptionsInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	CustomerID    string   `json:"customer_id" jsonschema:"RevenueCat customer ID (App User ID)."`
	Environment   string   `json:"environment,omitempty" jsonschema:"Optional store environment filter: production or sandbox."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand (for example items.product)."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type searchSubscriptionsInput struct {
	Project                     string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	StoreSubscriptionIdentifier string   `json:"store_subscription_identifier" jsonschema:"Store subscription transaction identifier (such as an Apple original_transaction_id, Google purchaseToken, or Stripe sub_... ID)."`
	IncludeScheduled            *bool    `json:"include_scheduled,omitempty" jsonschema:"Whether to include scheduled subscription changes."`
	Expand                      []string `json:"expand,omitempty" jsonschema:"Optional fields to expand."`
}

type getSubscriptionInput struct {
	Project        string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	SubscriptionID string   `json:"subscription_id" jsonschema:"RevenueCat subscription ID (sub...)."`
	Expand         []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: redemption."`
}

type listSubscriptionEntitlementsInput struct {
	Project        string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	SubscriptionID string `json:"subscription_id" jsonschema:"RevenueCat subscription ID (sub...)."`
	Limit          *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter  string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

func registerSubscriptionTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_customer_subscriptions",
		Description: "List subscriptions belonging to `customer_id`, optionally filtered by `environment` (production or sandbox).",
		Annotations: readOnly(),
	}, listCustomerSubscriptions)

	add(r, &mcp.Tool{
		Name:        "search_subscriptions",
		Description: "Search subscriptions in a RevenueCat project by `store_subscription_identifier`.",
		Annotations: readOnly(),
	}, searchSubscriptions)

	add(r, &mcp.Tool{
		Name:        "get_subscription",
		Description: "Get a RevenueCat subscription by `subscription_id`.",
		Annotations: readOnly(),
	}, getSubscription)

	add(r, &mcp.Tool{
		Name:        "list_subscription_entitlements",
		Description: "List entitlements unlocked by `subscription_id`.",
		Annotations: readOnly(),
	}, listSubscriptionEntitlements)
}

func listCustomerSubscriptions(ctx context.Context, env *Env, in listCustomerSubscriptionsInput) (*mcp.CallToolResult, any, error) {
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
		"/v2/projects/{project_id}/customers/{customer_id}/subscriptions",
		[]string{"projects", p.ProjectID, "customers", customerID, "subscriptions"},
		q,
	)
}

func searchSubscriptions(ctx context.Context, env *Env, in searchSubscriptionsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.SetRequiredString("store_subscription_identifier", in.StoreSubscriptionIdentifier); err != nil {
		return nil, nil, err
	}
	q.SetOptBool("include_scheduled", in.IncludeScheduled)
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/subscriptions", []string{"projects", p.ProjectID, "subscriptions"}, q)
}

func getSubscription(ctx context.Context, env *Env, in getSubscriptionInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	subID, err := requireID("subscription_id", in.SubscriptionID)
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
		"/v2/projects/{project_id}/subscriptions/{subscription_id}",
		[]string{"projects", p.ProjectID, "subscriptions", subID},
		q,
	)
}

func listSubscriptionEntitlements(ctx context.Context, env *Env, in listSubscriptionEntitlementsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	subID, err := requireID("subscription_id", in.SubscriptionID)
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
		"/v2/projects/{project_id}/subscriptions/{subscription_id}/entitlements",
		[]string{"projects", p.ProjectID, "subscriptions", subID, "entitlements"},
		q,
	)
}
