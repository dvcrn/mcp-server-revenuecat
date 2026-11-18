package server

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dvcrn/mcp-server-revenuecat-org/revenuecat"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// applyPagination sets limit and optional starting_after on q. When a caller
// passes the full next_page URL from a previous list response, the
// starting_after parameter is extracted from its query string.
func applyPagination(q *revenuecat.Query, limit *int, startingAfter string) error {
	n := defaultPageLimit
	if limit != nil {
		if *limit < 1 {
			return fmt.Errorf("limit must be at least 1 (default %d, max %d), got %d", defaultPageLimit, maxPageLimit, *limit)
		}
		n = min(*limit, maxPageLimit)
	}
	q.SetInt("limit", n)

	cursor, err := normalizeStartingAfter(startingAfter)
	if err != nil {
		return err
	}
	return q.SetString("starting_after", cursor)
}

func normalizeStartingAfter(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	if (strings.HasPrefix(s, "/v2/") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://")) && strings.Contains(s, "?") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("invalid starting_after cursor")
		}
		cursor := u.Query().Get("starting_after")
		if cursor == "" {
			return "", fmt.Errorf("next_page URL in starting_after has no starting_after query parameter")
		}
		return cursor, nil
	}

	return s, nil
}
