package server

import (
	"context"

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

type listOfferingsInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported values: items.package, items.package.product."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getOfferingInput struct {
	Project    string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID string   `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...)."`
	Expand     []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported values: package, package.product."`
}

type listOfferingPackagesInput struct {
	Project       string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID    string   `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...)."`
	Expand        []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: items.product."`
	Limit         *int     `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string   `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getPackageInput struct {
	Project   string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID string   `json:"package_id" jsonschema:"RevenueCat package ID (pkge...)."`
	Expand    []string `json:"expand,omitempty" jsonschema:"Optional fields to expand; supported value: product."`
}

type listPackageProductsInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID     string `json:"package_id" jsonschema:"RevenueCat package ID (pkge...)."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

func registerCatalogTools(r registry) {
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
		Name:        "list_offerings",
		Description: "List offerings in a RevenueCat project. Pass `expand: [\"items.package\", \"items.package.product\"]` to inline packages and products.",
		Annotations: readOnly(),
	}, listOfferings)

	add(r, &mcp.Tool{
		Name:        "get_offering",
		Description: "Get a RevenueCat offering by `offering_id`.",
		Annotations: readOnly(),
	}, getOffering)

	add(r, &mcp.Tool{
		Name:        "list_offering_packages",
		Description: "List packages inside `offering_id`.",
		Annotations: readOnly(),
	}, listOfferingPackages)

	add(r, &mcp.Tool{
		Name:        "get_package",
		Description: "Get a RevenueCat package by `package_id`.",
		Annotations: readOnly(),
	}, getPackage)

	add(r, &mcp.Tool{
		Name:        "list_package_products",
		Description: "List products attached to `package_id` with their eligibility criteria.",
		Annotations: readOnly(),
	}, listPackageProducts)
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

func listOfferings(ctx context.Context, env *Env, in listOfferingsInput) (*mcp.CallToolResult, any, error) {
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

	return getJSON(ctx, p, "/v2/projects/{project_id}/offerings", []string{"projects", p.ProjectID, "offerings"}, q)
}

func getOffering(ctx context.Context, env *Env, in getOfferingInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	offeringID, err := requireID("offering_id", in.OfferingID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/offerings/{offering_id}", []string{"projects", p.ProjectID, "offerings", offeringID}, q)
}

func listOfferingPackages(ctx context.Context, env *Env, in listOfferingPackagesInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	offeringID, err := requireID("offering_id", in.OfferingID)
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

	return getJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/offerings/{offering_id}/packages",
		[]string{"projects", p.ProjectID, "offerings", offeringID, "packages"},
		q,
	)
}

func getPackage(ctx context.Context, env *Env, in getPackageInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.AddStrings("expand", in.Expand); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/packages/{package_id}", []string{"projects", p.ProjectID, "packages", pkgID}, q)
}

func listPackageProducts(ctx context.Context, env *Env, in listPackageProductsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
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
		"/v2/projects/{project_id}/packages/{package_id}/products",
		[]string{"projects", p.ProjectID, "packages", pkgID, "products"},
		q,
	)
}
