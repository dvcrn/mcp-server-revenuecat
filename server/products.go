package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type listProductsInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	AppID         string   `json:"app_id,omitempty" jsonschema:"Optional app_id filter to return products from a single app."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported values: items.app, items.indicative_price."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getProductInput struct {
	Project   string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ProductID string   `json:"product_id" jsonschema:"RevenueCat product ID (prod...)."`
	Expand    []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported values: app, indicative_price."`
}

type createProductInput struct {
	Project              string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	StoreIdentifier      string `json:"store_identifier" jsonschema:"Store product identifier, such as com.example.pro.monthly."`
	AppID                string `json:"app_id" jsonschema:"RevenueCat app ID (app...) that owns this product."`
	Type                 string `json:"type" jsonschema:"Product type: subscription, one_time (Web Billing/Stripe), consumable, non_consumable, or non_renewing_subscription. Test Store apps accept subscription, consumable, and non_consumable."`
	DisplayName          string `json:"display_name,omitempty" jsonschema:"Optional internal display name."`
	SubscriptionDuration string `json:"subscription_duration,omitempty" jsonschema:"ISO 8601 duration (P1W, P1M, P2M, P3M, P6M, or P1Y), required when creating a Test Store subscription product."`
	Title                string `json:"title,omitempty" jsonschema:"User-facing product title, required for Test Store products."`
}

type deleteProductInput struct {
	Project   string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ProductID string `json:"product_id" jsonschema:"RevenueCat product ID (prod...) to delete."`
}

func registerProductTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_products",
		Description: "List store products configured in a RevenueCat project, optionally filtered by `app_id`.",
		Annotations: readOnly(),
	}, listProducts)

	add(r, &mcp.Tool{
		Name:        "get_product",
		Description: "Get a RevenueCat product by `product_id`.",
		Annotations: readOnly(),
	}, getProduct)

	add(r, &mcp.Tool{
		Name:        "create_product",
		Description: "Create a product in a RevenueCat project for `app_id` with `store_identifier` and `type`.",
		Annotations: mutating(),
	}, createProduct)

	add(r, &mcp.Tool{
		Name:        "delete_product",
		Description: "Delete a product from a RevenueCat project by `product_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, deleteProduct)
}

func listProducts(ctx context.Context, env *Env, in listProductsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}
	if err := q.SetString("app_id", in.AppID); err != nil {
		return nil, nil, err
	}
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/products", []string{"projects", p.ProjectID, "products"}, q)
}

func getProduct(ctx context.Context, env *Env, in getProductInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	productID, err := requireID("product_id", in.ProductID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/products/{product_id}", []string{"projects", p.ProjectID, "products", productID}, q)
}

func createProduct(ctx context.Context, env *Env, in createProductInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	storeID, err := requireCleanField("store_identifier", in.StoreIdentifier)
	if err != nil {
		return nil, nil, err
	}

	appID, err := requireCleanField("app_id", in.AppID)
	if err != nil {
		return nil, nil, err
	}

	prodType := strings.TrimSpace(in.Type)
	switch prodType {
	case "subscription", "one_time", "consumable", "non_consumable", "non_renewing_subscription":
	default:
		return nil, nil, fmt.Errorf("type must be one of subscription, one_time, consumable, non_consumable, or non_renewing_subscription")
	}

	body := map[string]any{
		"store_identifier": storeID,
		"app_id":           appID,
		"type":             prodType,
	}

	if strings.TrimSpace(in.DisplayName) != "" {
		dn, err := requireCleanField("display_name", in.DisplayName)
		if err != nil {
			return nil, nil, err
		}
		body["display_name"] = dn
	}

	if strings.TrimSpace(in.SubscriptionDuration) != "" {
		if prodType != "subscription" {
			return nil, nil, fmt.Errorf("subscription_duration is only valid when type is subscription")
		}
		dur, err := requireCleanField("subscription_duration", in.SubscriptionDuration)
		if err != nil {
			return nil, nil, err
		}
		body["subscription"] = map[string]any{"duration": dur}
	}

	if strings.TrimSpace(in.Title) != "" {
		title, err := requireCleanField("title", in.Title)
		if err != nil {
			return nil, nil, err
		}
		body["title"] = title
	}

	return postJSON(ctx, p, "/v2/projects/{project_id}/products", []string{"projects", p.ProjectID, "products"}, body)
}

func deleteProduct(ctx context.Context, env *Env, in deleteProductInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	productID, err := requireID("product_id", in.ProductID)
	if err != nil {
		return nil, nil, err
	}

	return deleteJSON(ctx, p, "/v2/projects/{project_id}/products/{product_id}", []string{"projects", p.ProjectID, "products", productID})
}
