package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

// minEpochMillis is 2001-09-09T01:46:40Z in Unix milliseconds, used to reject
// 10-digit Unix-second timestamps accidentally passed to millisecond fields.
const minEpochMillis int64 = 1_000_000_000_000

func postJSON(ctx context.Context, p *Project, endpoint string, segments []string, body any) (*mcp.CallToolResult, any, error) {
	resp, err := p.Client.Post(ctx, endpoint, segments, body)
	if err != nil {
		return nil, nil, err
	}

	return jsonBodyResult(resp), nil, nil
}

func deleteJSON(ctx context.Context, p *Project, endpoint string, segments []string) (*mcp.CallToolResult, any, error) {
	resp, err := p.Client.Delete(ctx, endpoint, segments)
	if err != nil {
		return nil, nil, err
	}

	return jsonBodyResult(resp), nil, nil
}

func requireEpochMillis(name string, ms int64) error {
	if ms < minEpochMillis {
		return fmt.Errorf("%s must be a Unix timestamp in milliseconds (>= %d)", name, minEpochMillis)
	}

	return nil
}

func requireCleanField(name, val string) (string, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	if revenuecat.HasControlOrInvalidUTF8(trimmed) {
		return "", fmt.Errorf("%s contains invalid characters", name)
	}

	return trimmed, nil
}

func requireIDList(name string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s must contain at least one ID", name)
	}

	out := make([]string, len(ids))
	for i, raw := range ids {
		cleaned, err := requireCleanField(name, raw)
		if err != nil {
			return nil, err
		}
		out[i] = cleaned
	}

	return out, nil
}

func requireChange(body map[string]any, fields ...string) error {
	if len(body) == 0 {
		return fmt.Errorf("pass at least one field to update (%s)", strings.Join(fields, ", "))
	}

	return nil
}
