package server

import (
	"context"
	"fmt"
	"strings"

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
		body,
	)
}
