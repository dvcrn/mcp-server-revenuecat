package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/dvcrn/mcp-server-revenuecat/revenuecat"
)

// Environment variables the server reads.
const (
	EnvProjects  = "REVENUECAT_PROJECTS"
	EnvToken     = "MCP_TOKEN"
	EnvReadToken = "MCP_READ_TOKEN"
	EnvReadOnly  = "MCP_READ_ONLY"
)

// Project is one configured RevenueCat project that tools may act on.
// The secret API key is bound inside Client and not retained on Project.
type Project struct {
	Alias     string
	Name      string
	ProjectID string
	Client    *revenuecat.Client
}

// Env is the per-request configuration tools read from the context.
type Env struct {
	Projects []Project
}

type projectConfig struct {
	Alias     string `json:"alias"`
	Name      string `json:"name"`
	ProjectID string `json:"project_id"`
	APIKey    string `json:"api_key"`
}

func (projectConfig) String() string   { return "projectConfig{[REDACTED]}" }
func (projectConfig) GoString() string { return "projectConfig{[REDACTED]}" }

var aliasPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// NewEnv builds an Env from REVENUECAT_PROJECTS JSON, validating every entry
// and binding a dedicated HTTP client per project.
func NewEnv(projectsJSON string, httpClient *http.Client, opts ...revenuecat.Option) (*Env, error) {
	projects, err := parseProjects(projectsJSON, httpClient, opts...)
	if err != nil {
		return nil, err
	}

	return &Env{Projects: projects}, nil
}

func parseProjects(raw string, httpClient *http.Client, opts ...revenuecat.Option) ([]Project, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is not set", EnvProjects)
	}

	var entries []projectConfig
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of {alias, name, project_id, api_key}: %w", EnvProjects, err)
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%s has no projects", EnvProjects)
	}

	seen := make(map[string]bool, len(entries)*2)
	projects := make([]Project, len(entries))
	for i, entry := range entries {
		if !aliasPattern.MatchString(entry.Alias) || strings.HasPrefix(entry.Alias, "sk_") {
			return nil, fmt.Errorf("%s: alias %q must start with a lowercase letter or digit, not start with sk_, and contain only lowercase letters, digits, - or _", EnvProjects, redactSecretLike(entry.Alias))
		}

		if !revenuecat.ValidProjectID(entry.ProjectID) {
			return nil, fmt.Errorf("%s: project_id %q for alias %q is not a proj... ID", EnvProjects, redactSecretLike(entry.ProjectID), entry.Alias)
		}

		if !revenuecat.ValidSecretKey(entry.APIKey) {
			return nil, fmt.Errorf("%s: api_key for alias %q must be a RevenueCat API v2 secret key starting with sk_", EnvProjects, entry.Alias)
		}

		if entry.Alias == entry.ProjectID || seen[entry.Alias] || seen[entry.ProjectID] {
			return nil, fmt.Errorf("%s: alias %q or project_id %q appears more than once", EnvProjects, entry.Alias, entry.ProjectID)
		}

		seen[entry.Alias] = true
		seen[entry.ProjectID] = true

		name := strings.TrimSpace(entry.Name)
		if name == "" {
			name = entry.Alias
		}

		projects[i] = Project{
			Alias:     entry.Alias,
			Name:      name,
			ProjectID: entry.ProjectID,
			Client:    revenuecat.New(httpClient, entry.APIKey, opts...),
		}
	}

	return projects, nil
}

// resolveProject returns the configured Project matching an alias or allowlisted proj... ID.
func (e *Env) resolveProject(ref string) (*Project, error) {
	for i := range e.Projects {
		p := &e.Projects[i]
		if ref == p.Alias || ref == p.ProjectID {
			return p, nil
		}
	}

	aliases := make([]string, len(e.Projects))
	for i, p := range e.Projects {
		aliases[i] = p.Alias
	}

	if ref == "" {
		return nil, fmt.Errorf("project is required; use one of: %s", strings.Join(aliases, ", "))
	}

	return nil, fmt.Errorf("unknown project %q; use one of: %s (see list_projects)", ref, strings.Join(aliases, ", "))
}

type envKey struct{}

// WithEnv returns a context whose tool calls use env.
func WithEnv(ctx context.Context, env *Env) context.Context {
	return context.WithValue(ctx, envKey{}, env)
}

var errNoEnv = errors.New("server is not configured")

func envFrom(ctx context.Context) (*Env, error) {
	env, ok := ctx.Value(envKey{}).(*Env)
	if !ok || env == nil {
		return nil, errNoEnv
	}

	return env, nil
}

func redactSecretLike(v string) string {
	if strings.HasPrefix(strings.TrimSpace(v), "sk_") {
		return "[REDACTED]"
	}

	return v
}
