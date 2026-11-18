package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

type listProjectsInput struct{}

type projectOutput struct {
	Alias     string `json:"alias"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
}

type listProjectsOutput struct {
	Projects []projectOutput `json:"projects"`
}

type listAppsInput struct {
	Project       string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	Limit         *int   `json:"limit,omitempty" jsonschema:"Maximum items to return (default 20, max 100)."`
	StartingAfter string `json:"starting_after,omitempty" jsonschema:"Pagination cursor from the previous page's next_page starting_after parameter."`
}

type getAppInput struct {
	Project string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	AppID   string `json:"app_id" jsonschema:"RevenueCat app ID (for example app1a2b3c4d)."`
}

func registerProjectTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "list_projects",
		Description: "List the configured RevenueCat projects this server can act on, with the alias and project_id accepted as `project` by all other tools.",
		Annotations: readOnly(),
	}, listProjects)

	add(r, &mcp.Tool{
		Name:        "list_apps",
		Description: "List apps in a RevenueCat project. Page through results with `starting_after` from `next_page`.",
		Annotations: readOnly(),
	}, listApps)

	add(r, &mcp.Tool{
		Name:        "get_app",
		Description: "Get a RevenueCat app by `app_id`, including its store configuration.",
		Annotations: readOnly(),
	}, getApp)
}

func listProjects(_ context.Context, env *Env, _ listProjectsInput) (*mcp.CallToolResult, listProjectsOutput, error) {
	out := listProjectsOutput{Projects: make([]projectOutput, len(env.Projects))}
	for i, p := range env.Projects {
		out.Projects[i] = projectOutput{
			Alias:     p.Alias,
			Name:      p.Name,
			ProjectID: p.ProjectID,
		}
	}

	return nil, out, nil
}

func listApps(ctx context.Context, env *Env, in listAppsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := applyPagination(q, in.Limit, in.StartingAfter); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/apps", []string{"projects", p.ProjectID, "apps"}, q)
}

func getApp(ctx context.Context, env *Env, in getAppInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	appID, err := requireID("app_id", in.AppID)
	if err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/apps/{app_id}", []string{"projects", p.ProjectID, "apps", appID}, nil)
}
