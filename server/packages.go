package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

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

type createOfferingPackageInput struct {
	Project     string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID  string `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...) that will contain the package."`
	LookupKey   string `json:"lookup_key" jsonschema:"Package identifier within the offering, such as $rc_monthly, $rc_annual, or a custom key."`
	DisplayName string `json:"display_name" jsonschema:"Human-readable package display name."`
	Position    *int   `json:"position,omitempty" jsonschema:"Optional 1-based display position within the offering."`
}

type updatePackageInput struct {
	Project     string  `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID   string  `json:"package_id" jsonschema:"RevenueCat package ID (pkge...)."`
	DisplayName *string `json:"display_name,omitempty" jsonschema:"Updated human-readable package display name."`
	Position    *int    `json:"position,omitempty" jsonschema:"Updated 1-based display position within the offering."`
}

type deletePackageInput struct {
	Project   string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID string `json:"package_id" jsonschema:"RevenueCat package ID (pkge...) to delete."`
}

type packageProductAssociation struct {
	ProductID           string `json:"product_id" jsonschema:"RevenueCat product ID (prod...) to attach."`
	EligibilityCriteria string `json:"eligibility_criteria,omitempty" jsonschema:"Eligibility criteria: all (default), google_sdk_lt_6, or google_sdk_ge_6."`
}

type attachPackageProductsInput struct {
	Project   string                      `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID string                      `json:"package_id" jsonschema:"RevenueCat package ID (pkge...)."`
	Products  []packageProductAssociation `json:"products" jsonschema:"Products and eligibility criteria to attach to this package."`
}

type detachPackageProductsInput struct {
	Project    string   `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	PackageID  string   `json:"package_id" jsonschema:"RevenueCat package ID (pkge...)."`
	ProductIDs []string `json:"product_ids" jsonschema:"Product IDs (prod...) to detach from this package."`
}

func registerPackageTools(r registry) {
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

	add(r, &mcp.Tool{
		Name:        "create_offering_package",
		Description: "Create a package inside `offering_id` with `lookup_key`, `display_name`, and optional `position`.",
		Annotations: mutating(),
	}, createOfferingPackage)

	add(r, &mcp.Tool{
		Name:        "update_package",
		Description: "Update a package's `display_name` or `position` by `package_id`.",
		Annotations: idempotent(),
	}, updatePackage)

	add(r, &mcp.Tool{
		Name:        "delete_package",
		Description: "Delete a package from its offering by `package_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, deletePackage)

	add(r, &mcp.Tool{
		Name:        "attach_package_products",
		Description: "Attach `products` (with `eligibility_criteria`, defaulting to `all`) to `package_id`.",
		Annotations: mutating(),
	}, attachPackageProducts)

	add(r, &mcp.Tool{
		Name:        "detach_package_products",
		Description: "Detach `product_ids` from `package_id` in its offering. Confirm with the user first.",
		Annotations: destructive(),
	}, detachPackageProducts)
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

func createOfferingPackage(ctx context.Context, env *Env, in createOfferingPackageInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	offeringID, err := requireID("offering_id", in.OfferingID)
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

	body := map[string]any{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	if in.Position != nil {
		if *in.Position < 1 {
			return nil, nil, fmt.Errorf("position must be at least 1")
		}
		body["position"] = *in.Position
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/offerings/{offering_id}/packages",
		[]string{"projects", p.ProjectID, "offerings", offeringID, "packages"},
		body,
	)
}

func updatePackage(ctx context.Context, env *Env, in updatePackageInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
	if err != nil {
		return nil, nil, err
	}

	body := make(map[string]any, 2)
	if in.DisplayName != nil {
		dn, err := requireCleanField("display_name", *in.DisplayName)
		if err != nil {
			return nil, nil, err
		}
		body["display_name"] = dn
	}
	if in.Position != nil {
		if *in.Position < 1 {
			return nil, nil, fmt.Errorf("position must be at least 1")
		}
		body["position"] = *in.Position
	}

	if err := requireChange(body, "display_name", "position"); err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/packages/{package_id}",
		[]string{"projects", p.ProjectID, "packages", pkgID},
		body,
	)
}

func deletePackage(ctx context.Context, env *Env, in deletePackageInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
	if err != nil {
		return nil, nil, err
	}

	return deleteJSON(ctx, p, "/v2/projects/{project_id}/packages/{package_id}", []string{"projects", p.ProjectID, "packages", pkgID})
}

func attachPackageProducts(ctx context.Context, env *Env, in attachPackageProductsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
	if err != nil {
		return nil, nil, err
	}

	if len(in.Products) == 0 {
		return nil, nil, fmt.Errorf("products must contain at least one item")
	}

	items := make([]map[string]any, len(in.Products))
	for i, assoc := range in.Products {
		prodID, err := requireCleanField("products.product_id", assoc.ProductID)
		if err != nil {
			return nil, nil, err
		}

		criteria := strings.TrimSpace(assoc.EligibilityCriteria)
		if criteria == "" {
			criteria = "all"
		}
		switch criteria {
		case "all", "google_sdk_lt_6", "google_sdk_ge_6":
		default:
			return nil, nil, fmt.Errorf("eligibility_criteria must be one of all, google_sdk_lt_6, or google_sdk_ge_6")
		}

		items[i] = map[string]any{
			"product_id":           prodID,
			"eligibility_criteria": criteria,
		}
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/packages/{package_id}/actions/attach_products",
		[]string{"projects", p.ProjectID, "packages", pkgID, "actions", "attach_products"},
		map[string]any{"products": items},
	)
}

func detachPackageProducts(ctx context.Context, env *Env, in detachPackageProductsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	pkgID, err := requireID("package_id", in.PackageID)
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
		"/v2/projects/{project_id}/packages/{package_id}/actions/detach_products",
		[]string{"projects", p.ProjectID, "packages", pkgID, "actions", "detach_products"},
		map[string]any{"product_ids": productIDs},
	)
}
