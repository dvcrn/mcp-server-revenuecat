//go:build e2e

package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/internal/httpclient"
)

var (
	e2eSecretPattern = regexp.MustCompile(`sk_[!-~]+`)
	e2eProjPattern   = regexp.MustCompile(`\bproj[0-9a-zA-Z_-]{4,}\b`)
	e2eAppPattern    = regexp.MustCompile(`\bapp[0-9a-f]{6,}\b`)
)

func redactE2E(s string) string {
	s = e2eSecretPattern.ReplaceAllString(s, "[REDACTED_KEY]")
	s = e2eProjPattern.ReplaceAllString(s, "proj_REDACTED")
	return e2eAppPattern.ReplaceAllString(s, "app_REDACTED")
}

func randomHex(t *testing.T, n int) string {
	t.Helper()
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return hex.EncodeToString(buf)
}

func toolText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

func callToolJSON(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()

	callCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res, err := session.CallTool(callCtx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("%s protocol error: %s", name, redactE2E(err.Error()))
	}
	if res.IsError {
		t.Fatalf("%s returned tool error: %s", name, redactE2E(toolText(res)))
	}
	raw := toolText(res)
	if raw == "" {
		t.Fatalf("%s returned empty text content", name)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("%s returned invalid JSON: %v", name, err)
	}
	return decoded
}

func connectLiveSession(t *testing.T, sessionCtx context.Context, srv *mcp.Server) *mcp.ClientSession {
	t.Helper()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(sessionCtx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server Connect: %s", redactE2E(err.Error()))
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "1.0"}, nil)
	session, err := client.Connect(sessionCtx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %s", redactE2E(err.Error()))
	}
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestLiveE2E(t *testing.T) {
	rawProjects := strings.TrimSpace(os.Getenv(EnvProjects))
	if rawProjects == "" {
		t.Skipf("%s is not set; skipping live RevenueCat API v2 E2E test", EnvProjects)
	}

	env, err := NewEnv(rawProjects, httpclient.New())
	if err != nil {
		t.Fatalf("NewEnv: %s", redactE2E(err.Error()))
	}

	// Register session cancellation before per-resource t.Cleanup callbacks so LIFO
	// cleanup runs delete_customer before closing the MCP sessions.
	sessionCtx, cancelSessions := context.WithCancel(WithEnv(context.Background(), env))
	t.Cleanup(cancelSessions)

	roSession := connectLiveSession(t, sessionCtx, NewReadOnly())
	fullSession := connectLiveSession(t, sessionCtx, New())

	targetOverride := strings.TrimSpace(os.Getenv("REVENUECAT_E2E_PROJECT"))
	readProject := targetOverride
	if readProject == "" {
		readProject = env.Projects[0].Alias
	}

	t.Run("read_only_and_read_tools", func(t *testing.T) {
		toolsRes, err := roSession.ListTools(sessionCtx, nil)
		if err != nil {
			t.Fatalf("ListTools: %s", redactE2E(err.Error()))
		}
		if len(toolsRes.Tools) != expectedReadToolCount {
			t.Fatalf("read-only tool count = %d, want %d", len(toolsRes.Tools), expectedReadToolCount)
		}

		if _, err := roSession.CallTool(sessionCtx, &mcp.CallToolParams{
			Name:      "create_customer",
			Arguments: map[string]any{"project": readProject, "id": "mcp_e2e_blocked"},
		}); err == nil {
			t.Fatal("read-only session allowed create_customer")
		}

		projectsObj := callToolJSON(t, sessionCtx, roSession, "list_projects", nil)
		projectsList, _ := projectsObj["projects"].([]any)
		if len(projectsList) == 0 {
			t.Fatal("list_projects returned zero configured projects")
		}

		for _, toolName := range []string{
			"list_apps",
			"get_overview_metrics",
			"list_customers",
			"list_products",
			"list_entitlements",
			"list_offerings",
		} {
			got := callToolJSON(t, sessionCtx, roSession, toolName, map[string]any{"project": readProject})
			if got["object"] == nil {
				t.Errorf("%s response missing object field", toolName)
			}
		}
	})

	t.Run("sandbox_customer_lifecycle", func(t *testing.T) {
		writeProject := targetOverride
		if writeProject == "" {
			if len(env.Projects) != 1 {
				t.Skip("multiple projects configured; set REVENUECAT_E2E_PROJECT to run live write lifecycle")
			}
			writeProject = env.Projects[0].Alias
		}

		custID := fmt.Sprintf("mcp_e2e_%d_%s", time.Now().Unix(), randomHex(t, 4))
		mayExist := false
		deleted := false
		t.Cleanup(func() {
			if !mayExist || deleted {
				return
			}
			cleanCtx, cancel := context.WithTimeout(sessionCtx, 15*time.Second)
			defer cancel()
			res, err := fullSession.CallTool(cleanCtx, &mcp.CallToolParams{
				Name:      "delete_customer",
				Arguments: map[string]any{"project": writeProject, "customer_id": custID},
			})
			if err != nil {
				t.Errorf("cleanup delete_customer(%q) error: %s", custID, redactE2E(err.Error()))
				return
			}
			if res.IsError {
				msg := toolText(res)
				if !strings.Contains(msg, "revenuecat: HTTP 404") {
					t.Errorf("cleanup delete_customer(%q) failed: %s", custID, redactE2E(msg))
				}
			}
		})

		mayExist = true
		createCtx, cancelCreate := context.WithTimeout(sessionCtx, 20*time.Second)
		createRes, err := fullSession.CallTool(createCtx, &mcp.CallToolParams{
			Name: "create_customer",
			Arguments: map[string]any{
				"project": writeProject,
				"id":      custID,
				"attributes": []map[string]any{
					{"name": "$displayName", "value": "MCP E2E Test Customer"},
				},
			},
		})
		cancelCreate()
		if err != nil {
			t.Fatalf("create_customer protocol error: %s", redactE2E(err.Error()))
		}
		if createRes.IsError {
			msg := toolText(createRes)
			if strings.Contains(msg, "revenuecat: HTTP 4") {
				mayExist = false
			}
			t.Fatalf("create_customer returned tool error: %s", redactE2E(msg))
		}
		var created map[string]any
		if err := json.Unmarshal([]byte(toolText(createRes)), &created); err != nil {
			t.Fatalf("create_customer invalid JSON: %v", err)
		}
		if created["id"] != custID {
			t.Fatalf("create_customer id = %v, want %q", created["id"], custID)
		}

		fetched := callToolJSON(t, sessionCtx, roSession, "get_customer", map[string]any{
			"project":     writeProject,
			"customer_id": custID,
			"expand":      []string{"attributes"},
		})
		if fetched["id"] != custID {
			t.Fatalf("get_customer id = %v, want %q", fetched["id"], custID)
		}

		callToolJSON(t, sessionCtx, fullSession, "set_customer_attributes", map[string]any{
			"project":     writeProject,
			"customer_id": custID,
			"attributes": []map[string]any{
				{"name": "e2e_stage", "value": "updated"},
			},
		})

		attrs := callToolJSON(t, sessionCtx, roSession, "list_customer_attributes", map[string]any{
			"project":     writeProject,
			"customer_id": custID,
		})
		items, _ := attrs["items"].([]any)
		if len(items) == 0 {
			t.Fatal("list_customer_attributes returned empty items after set_customer_attributes")
		}

		callToolJSON(t, sessionCtx, fullSession, "delete_customer", map[string]any{
			"project":     writeProject,
			"customer_id": custID,
		})
		deleted = true

		// RevenueCat customer reads are eventually consistent across replicas briefly after DELETE.
		var (
			afterDel *mcp.CallToolResult
			afterMsg string
		)
		for attempt := range 8 {
			if attempt > 0 {
				time.Sleep(250 * time.Millisecond)
			}
			verifyCtx, cancel := context.WithTimeout(sessionCtx, 15*time.Second)
			afterDel, err = roSession.CallTool(verifyCtx, &mcp.CallToolParams{
				Name:      "get_customer",
				Arguments: map[string]any{"project": writeProject, "customer_id": custID},
			})
			cancel()
			if err != nil {
				t.Fatalf("get_customer after delete protocol error: %s", redactE2E(err.Error()))
			}
			afterMsg = toolText(afterDel)
			if afterDel.IsError && strings.Contains(afterMsg, "revenuecat: HTTP 404") {
				return
			}
		}
		t.Fatalf("get_customer(%q) after delete = (isError=%v, %s), want HTTP 404", custID, afterDel.IsError, redactE2E(afterMsg))
	})
}
