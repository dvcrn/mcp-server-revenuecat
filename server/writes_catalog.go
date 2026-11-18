package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type createProductInput struct {
	Project              string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	StoreIdentifier      string `json:"store_identifier" jsonschema:"Store product identifier, such as com.example.pro.monthly."`
	AppID                string `json:"app_id" jsonschema:"RevenueCat app ID (app...) that owns this product."`
	Type                 string `json:"type" jsonschema:"Product type: subscription, one_time, consumable, non_consumable, or non_renewing_subscription."`
	DisplayName          string `json:"display_name,omitempty" jsonschema:"Optional human-readable display name."`
	SubscriptionDuration string `json:"subscription_duration,omitempty" jsonschema:"Optional ISO 8601 duration (such as P1W, P1M, P3M, P6M, P1Y) when creating a Test Store subscription product."`
	Title                string `json:"title,omitempty" jsonschema:"Optional user-facing title for Test Store products."`
}

type deleteProductInput struct {
	Project   string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ProductID string `json:"product_id" jsonschema:"RevenueCat product ID (prod...) to delete."`
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

type createOfferingInput struct {
	Project     string         `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	LookupKey   string         `json:"lookup_key" jsonschema:"Unique offering identifier, such as default."`
	DisplayName string         `json:"display_name" jsonschema:"Human-readable offering display name."`
	Metadata    map[string]any `json:"metadata,omitempty" jsonschema:"Optional custom key-value metadata object attached to the offering."`
}

type updateOfferingInput struct {
	Project       string         `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID    string         `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...)."`
	DisplayName   *string        `json:"display_name,omitempty" jsonschema:"Updated human-readable display name."`
	IsCurrent     *bool          `json:"is_current,omitempty" jsonschema:"Whether this offering is the default offering returned to apps."`
	Metadata      map[string]any `json:"metadata,omitempty" jsonschema:"Replacement custom metadata object. Mutually exclusive with clear_metadata."`
	ClearMetadata bool           `json:"clear_metadata,omitempty" jsonschema:"Whether to clear custom metadata on this offering."`
}

type deleteOfferingInput struct {
	Project    string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID string `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...) to delete."`
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

func registerCatalogWriteTools(r registry) {
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

	add(r, &mcp.Tool{
		Name:        "create_offering",
		Description: "Create an offering in a RevenueCat project with `lookup_key`, `display_name`, and optional `metadata`.",
		Annotations: mutating(),
	}, createOffering)

	add(r, &mcp.Tool{
		Name:        "update_offering",
		Description: "Update an offering's `display_name`, `is_current` flag, or `metadata`. Changing `is_current` switches the default offering returned to apps, so confirm with the user first.",
		Annotations: destructive(),
	}, updateOffering)

	add(r, &mcp.Tool{
		Name:        "delete_offering",
		Description: "Delete an offering and its packages from a RevenueCat project by `offering_id`. Confirm with the user first.",
		Annotations: destructive(),
	}, deleteOffering)

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

	return postJSON(ctx, p, "/v2/projects/{project_id}/products", []string{"projects", p.ProjectID, "products"}, nil, body)
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
		nil,
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
		nil,
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
		nil,
		map[string]any{"product_ids": productIDs},
	)
}

func createOffering(ctx context.Context, env *Env, in createOfferingInput) (*mcp.CallToolResult, any, error) {
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

	body := map[string]any{
		"lookup_key":   lookupKey,
		"display_name": displayName,
	}
	if in.Metadata != nil {
		body["metadata"] = in.Metadata
	}

	return postJSON(ctx, p, "/v2/projects/{project_id}/offerings", []string{"projects", p.ProjectID, "offerings"}, nil, body)
}

func updateOffering(ctx context.Context, env *Env, in updateOfferingInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	offeringID, err := requireID("offering_id", in.OfferingID)
	if err != nil {
		return nil, nil, err
	}

	if in.ClearMetadata && in.Metadata != nil {
		return nil, nil, fmt.Errorf("pass either metadata or clear_metadata: true, not both")
	}

	body := make(map[string]any, 3)
	if in.DisplayName != nil {
		dn, err := requireCleanField("display_name", *in.DisplayName)
		if err != nil {
			return nil, nil, err
		}
		body["display_name"] = dn
	}
	if in.IsCurrent != nil {
		body["is_current"] = *in.IsCurrent
	}
	if in.ClearMetadata {
		body["metadata"] = nil
	} else if in.Metadata != nil {
		body["metadata"] = in.Metadata
	}

	if err := requireChange(body, "display_name", "is_current", "metadata", "clear_metadata"); err != nil {
		return nil, nil, err
	}

	return postJSON(
		ctx,
		p,
		"/v2/projects/{project_id}/offerings/{offering_id}",
		[]string{"projects", p.ProjectID, "offerings", offeringID},
		nil,
		body,
	)
}

func deleteOffering(ctx context.Context, env *Env, in deleteOfferingInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	offeringID, err := requireID("offering_id", in.OfferingID)
	if err != nil {
		return nil, nil, err
	}

	return deleteJSON(ctx, p, "/v2/projects/{project_id}/offerings/{offering_id}", []string{"projects", p.ProjectID, "offerings", offeringID})
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
		nil,
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
		nil,
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
		nil,
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
		nil,
		map[string]any{"product_ids": productIDs},
	)
}
