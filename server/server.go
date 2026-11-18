// Package server implements the RevenueCat multi-project MCP server: tool
// registrations, HTTP auth, read-only gating, and context wiring shared by the
// native binary and the Cloudflare Worker.
package server

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

var (
	fullServer     = sync.OnceValue(func() *mcp.Server { return newServer(false) })
	readOnlyServer = sync.OnceValue(func() *mcp.Server { return newServer(true) })
)

// New returns the MCP server with every read and write tool. Tool schemas are
// built once per process (one Workers isolate); per-request config comes from
// the context via WithEnv.
func New() *mcp.Server { return fullServer() }

// NewReadOnly returns the MCP server with only the tools annotated read-only.
func NewReadOnly() *mcp.Server { return readOnlyServer() }

// StdioServer returns the server for stdio transport: the read-only one when
// MCP_READ_ONLY is set to true, otherwise the full one.
func StdioServer(getenv func(string) string) (*mcp.Server, error) {
	readOnly, err := readOnlyFromEnv(getenv)
	if err != nil {
		return nil, err
	}

	if readOnly {
		return NewReadOnly(), nil
	}

	return New(), nil
}

// readOnlyFromEnv reports whether MCP_READ_ONLY limits the server to read-only
// tools. A value that is not a boolean is an error, so a typo cannot silently
// allow writes.
func readOnlyFromEnv(getenv func(string) string) (bool, error) {
	v := strings.TrimSpace(getenv(EnvReadOnly))
	if v == "" {
		return false, nil
	}

	readOnly, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean such as 1 or 0, got %q", EnvReadOnly, v)
	}

	return readOnly, nil
}

func newServer(readOnly bool) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "revenuecat-org", Version: "0.1.0"}, nil)
	registerTools(registry{s: s, readOnly: readOnly})

	return s
}

// registry is the server tools are added to. A read-only registry skips every
// tool not annotated read-only.
type registry struct {
	s        *mcp.Server
	readOnly bool
}

func registerTools(r registry) {
	registerProjectTools(r)
	registerMetricTools(r)
	registerCustomerTools(r)
	registerCustomerWriteTools(r)
	registerSubscriptionTools(r)
	registerPurchaseTools(r)
	registerProductTools(r)
	registerEntitlementTools(r)
	registerOfferingTools(r)
	registerPackageTools(r)
}

// add registers a tool whose handler receives the request's Env. An Out of any
// gives the tool no structured output schema.
func add[In, Out any](r registry, tool *mcp.Tool, h func(context.Context, *Env, In) (*mcp.CallToolResult, Out, error)) {
	if tool.Annotations == nil {
		panic(fmt.Sprintf("tool %s: missing annotations", tool.Name))
	}
	if r.readOnly && !tool.Annotations.ReadOnlyHint {
		return
	}

	t := *tool
	t.InputSchema = schemaFor[In](t.Name)
	if reflect.TypeFor[Out]().Kind() != reflect.Interface {
		t.OutputSchema = schemaFor[Out](t.Name)
	}

	enforceReadOnly := r.readOnly || t.Annotations.ReadOnlyHint
	mcp.AddTool(r.s, &t, func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
		env, err := envFrom(ctx)
		if err != nil {
			var zero Out
			return nil, zero, err
		}

		if enforceReadOnly {
			ctx = revenuecat.WithReadOnly(ctx)
		}

		return h(ctx, env, in)
	})
}

// schemaFor infers T's schema and drops "null" from nilable fields (slices,
// maps, pointers), which tools always fill or omit and which some clients read
// as a different kind of parameter.
func schemaFor[T any](tool string) *jsonschema.Schema {
	s, err := jsonschema.For[T](nil)
	if err != nil {
		panic(fmt.Sprintf("tool %s: %v", tool, err))
	}

	dropNull(s)

	return s
}

func dropNull(s *jsonschema.Schema) {
	if s == nil {
		return
	}

	if types := slices.DeleteFunc(slices.Clone(s.Types), func(t string) bool { return t == "null" }); len(types) != len(s.Types) {
		s.Types = nil
		if len(types) == 1 {
			s.Type = types[0]
		} else {
			s.Types = types
		}
	}

	dropNull(s.Items)
	dropNull(s.AdditionalProperties)
	for _, p := range s.Properties {
		dropNull(p)
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func jsonBodyResult(body []byte) *mcp.CallToolResult {
	if len(bytes.TrimSpace(body)) == 0 {
		return textResult(`{"ok":true}`)
	}

	return textResult(string(body))
}

func getJSON(ctx context.Context, p *Project, endpoint string, segments []string, q *revenuecat.Query) (*mcp.CallToolResult, any, error) {
	vals := revenuecat.NewQuery().Values()
	if q != nil {
		vals = q.Values()
	}

	body, err := p.Client.Get(ctx, endpoint, segments, vals)
	if err != nil {
		return nil, nil, err
	}

	return jsonBodyResult(body), nil, nil
}

func requireID(name, val string) (string, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", name)
	}

	return trimmed, nil
}

func readOnly() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: new(true)}
}

func mutating() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: new(false), OpenWorldHint: new(true)}
}

// idempotent marks a non-destructive write that has no further effect when repeated with the same arguments.
func idempotent() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: new(false), IdempotentHint: true, OpenWorldHint: new(true)}
}

func destructive() *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{DestructiveHint: new(true), OpenWorldHint: new(true)}
}
