package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestNewEnvValidAndProjectResolution(t *testing.T) {
	t.Parallel()

	raw := `[
		{"alias": "alpha", "name": "Alpha App", "project_id": "proj1ab2c3d4", "api_key": "sk_dummy_alpha"},
		{"alias": "beta_2", "name": "", "project_id": "proj5ef6g7h8", "api_key": "sk_dummy_beta"}
	]`

	env, err := NewEnv(raw, http.DefaultClient)
	if err != nil {
		t.Fatalf("NewEnv returned unexpected error: %v", err)
	}
	if len(env.Projects) != 2 {
		t.Fatalf("len(Projects) = %d, want 2", len(env.Projects))
	}
	if env.Projects[0].Name != "Alpha App" {
		t.Errorf("Projects[0].Name = %q, want %q", env.Projects[0].Name, "Alpha App")
	}
	if env.Projects[1].Name != "beta_2" {
		t.Errorf("Projects[1].Name = %q, want fallback alias %q", env.Projects[1].Name, "beta_2")
	}

	byAlias, err := env.resolveProject("alpha")
	if err != nil || byAlias.ProjectID != "proj1ab2c3d4" {
		t.Fatalf("resolveProject(alpha) = (%v, %v), want proj1ab2c3d4", byAlias, err)
	}

	byID, err := env.resolveProject("proj5ef6g7h8")
	if err != nil || byID.Alias != "beta_2" {
		t.Fatalf("resolveProject(proj5ef6g7h8) = (%v, %v), want beta_2", byID, err)
	}

	if _, err := env.resolveProject(""); err == nil || !strings.Contains(err.Error(), "project is required") {
		t.Errorf("resolveProject(\"\") error = %v, want 'project is required'", err)
	}

	if _, err := env.resolveProject("unknown"); err == nil || !strings.Contains(err.Error(), "alpha, beta_2") {
		t.Errorf("resolveProject(unknown) error = %v, want configured aliases listed", err)
	}

	formatted := fmt.Sprintf("%v %#v", env, env)
	if strings.Contains(formatted, "sk_dummy_") {
		t.Fatalf("formatted Env leaked dummy secret key: %s", formatted)
	}
}

func TestNewEnvValidationAndSecretRedaction(t *testing.T) {
	t.Parallel()

	const dummySecret = "sk_dummy_never_echo_me"

	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "empty env",
			raw:  "   ",
			want: "REVENUECAT_PROJECTS is not set",
		},
		{
			name: "invalid json",
			raw:  `not-json`,
			want: "must be a JSON array",
		},
		{
			name: "empty array",
			raw:  `[]`,
			want: "has no projects",
		},
		{
			name: "uppercase alias",
			raw:  `[{"alias":"Alpha","project_id":"proj1ab2c3d4","api_key":"` + dummySecret + `"}]`,
			want: `alias "Alpha"`,
		},
		{
			name: "invalid project id",
			raw:  `[{"alias":"alpha","project_id":"acct_123","api_key":"` + dummySecret + `"}]`,
			want: `not a proj... ID`,
		},
		{
			name: "public key rejected",
			raw:  `[{"alias":"alpha","project_id":"proj1ab2c3d4","api_key":"appl_dummy_public"}]`,
			want: "starting with sk_",
		},
		{
			name: "whitespace in secret key rejected",
			raw:  `[{"alias":"alpha","project_id":"proj1ab2c3d4","api_key":"` + dummySecret + ` extra"}]`,
			want: "starting with sk_",
		},
		{
			name: "duplicate alias",
			raw:  `[{"alias":"alpha","project_id":"proj1ab2c3d4","api_key":"` + dummySecret + `"},{"alias":"alpha","project_id":"proj5ef6g7h8","api_key":"sk_dummy_second"}]`,
			want: "appears more than once",
		},
		{
			name: "duplicate project_id",
			raw:  `[{"alias":"alpha","project_id":"proj1ab2c3d4","api_key":"` + dummySecret + `"},{"alias":"beta","project_id":"proj1ab2c3d4","api_key":"sk_dummy_second"}]`,
			want: "appears more than once",
		},
		{
			name: "alias equal to project_id",
			raw:  `[{"alias":"proj1ab2c3d4","project_id":"proj1ab2c3d4","api_key":"` + dummySecret + `"}]`,
			want: "appears more than once",
		},
		{
			name: "secret pasted into alias is redacted",
			raw:  `[{"alias":"` + dummySecret + `","project_id":"proj1ab2c3d4","api_key":"sk_dummy_other"}]`,
			want: `alias "[REDACTED]"`,
		},
		{
			name: "secret pasted into project_id is redacted",
			raw:  `[{"alias":"alpha","project_id":"` + dummySecret + `","api_key":"sk_dummy_other"}]`,
			want: `project_id "[REDACTED]"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewEnv(tc.raw, http.DefaultClient)
			if err == nil {
				t.Fatalf("NewEnv(%s) succeeded, want error containing %q", tc.name, tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.want)
			}
			if strings.Contains(err.Error(), dummySecret) || strings.Contains(err.Error(), "appl_dummy_public") {
				t.Errorf("error leaked secret value: %q", err.Error())
			}
		})
	}

	cfg := projectConfig{Alias: "alpha", ProjectID: "proj1ab2c3d4", APIKey: dummySecret}
	if strings.Contains(fmt.Sprintf("%s %#v", cfg, cfg), dummySecret) {
		t.Fatal("projectConfig formatting leaked APIKey")
	}
}
