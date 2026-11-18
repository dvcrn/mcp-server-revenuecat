package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

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

type createOfferingInput struct {
	Project     string         `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	LookupKey   string         `json:"lookup_key" jsonschema:"Unique offering identifier, such as default."`
	DisplayName string         `json:"display_name" jsonschema:"Human-readable offering display name."`
	Metadata    map[string]any `json:"metadata,omitempty" jsonschema:"Optional custom key-value metadata object attached to the offering."`
}

type updateOfferingInput struct {
	Project       string         `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID    string         `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...)."`
	DisplayName   *string        `json:"display_name,omitempty" jsonschema:"Updated human-readable display name (must be unique within the project)."`
	IsCurrent     *bool          `json:"is_current,omitempty" jsonschema:"Set true to promote this offering to the default offering returned to apps (false is rejected; promote another offering instead)."`
	Metadata      map[string]any `json:"metadata,omitempty" jsonschema:"Replacement custom metadata object. Mutually exclusive with clear_metadata."`
	ClearMetadata bool           `json:"clear_metadata,omitempty" jsonschema:"Whether to clear custom metadata on this offering."`
}

type deleteOfferingInput struct {
	Project    string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	OfferingID string `json:"offering_id" jsonschema:"RevenueCat offering ID (ofrng...) to delete."`
}

func registerOfferingTools(r registry) {
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
		Name:        "create_offering",
		Description: "Create an offering in a RevenueCat project with `lookup_key`, `display_name`, and optional `metadata`.",
		Annotations: mutating(),
	}, createOffering)

	add(r, &mcp.Tool{
		Name:        "update_offering",
		Description: "Update an offering's `display_name`, promote it with `is_current: true`, or set/clear `metadata`. Setting `is_current: true` switches the default offering returned to apps, so confirm with the user first.",
		Annotations: destructive(),
	}, updateOffering)

	add(r, &mcp.Tool{
		Name:        "delete_offering",
		Description: "Delete a non-default offering and its packages by `offering_id`. If the offering has `is_current: true`, promote another offering with `update_offering` first. Confirm with the user first.",
		Annotations: destructive(),
	}, deleteOffering)
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

	return postJSON(ctx, p, "/v2/projects/{project_id}/offerings", []string{"projects", p.ProjectID, "offerings"}, body)
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
		if !*in.IsCurrent {
			return nil, nil, fmt.Errorf("is_current cannot be set to false; set is_current: true on another offering to switch the default offering")
		}
		body["is_current"] = true
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
