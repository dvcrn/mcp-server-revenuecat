package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

const dummyProjectsJSON = `[{"alias":"alpha","name":"Alpha","project_id":"proj1ab2c3d4","api_key":"sk_dummy_alpha"}]`

func TestHTTPHandlerAuthAndHealth(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		EnvProjects: dummyProjectsJSON,
		EnvToken:    "dummy-mcp-token",
	}
	h := Handler(func(k string) string { return vars[k] })

	t.Run("health requires no auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "ok\n" {
			t.Fatalf("GET /health = (%d, %q), want (200, \"ok\\n\")", rec.Code, rec.Body.String())
		}
	})

	initPayload := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)

	authCases := []struct {
		name   string
		header string
		value  string
		want   int
	}{
		{name: "missing token", want: http.StatusUnauthorized},
		{name: "wrong bearer", header: "Authorization", value: "Bearer wrong", want: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Authorization", value: "Basic dummy-mcp-token", want: http.StatusUnauthorized},
		{name: "empty bearer", header: "Authorization", value: "Bearer   ", want: http.StatusUnauthorized},
		{name: "valid bearer case-insensitive", header: "Authorization", value: "bearer dummy-mcp-token", want: http.StatusOK},
		{name: "valid X-API-Key", header: "X-API-Key", value: "dummy-mcp-token", want: http.StatusOK},
	}

	for _, tc := range authCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(initPayload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusUnauthorized {
				if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
					t.Errorf("WWW-Authenticate = %q, want Bearer", got)
				}
				if got := rec.Header().Get("Cache-Control"); got != "no-store" {
					t.Errorf("Cache-Control = %q, want no-store", got)
				}
			}
		})
	}
}

func TestEnvCacheReloadAndConcurrency(t *testing.T) {
	t.Parallel()

	var current atomic.Value
	current.Store(dummyProjectsJSON)

	cache := &envCache{}
	getenv := func(k string) string {
		if k == EnvProjects {
			return current.Load().(string)
		}
		return ""
	}

	first, err := cache.get(getenv)
	if err != nil {
		t.Fatalf("first cache.get error: %v", err)
	}
	second, err := cache.get(getenv)
	if err != nil {
		t.Fatalf("second cache.get error: %v", err)
	}
	if first != second {
		t.Fatal("expected envCache to reuse parsed *Env when REVENUECAT_PROJECTS is unchanged")
	}

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			got, err := cache.get(getenv)
			if err != nil || got != first {
				t.Errorf("concurrent cache.get = (%p, %v), want (%p, nil)", got, err, first)
			}
		})
	}
	wg.Wait()

	updatedJSON := `[{"alias":"gamma","name":"Gamma","project_id":"proj9ij0k1l2","api_key":"sk_dummy_gamma"}]`
	current.Store(updatedJSON)

	reloaded, err := cache.get(getenv)
	if err != nil {
		t.Fatalf("reloaded cache.get error: %v", err)
	}
	if reloaded == first || reloaded.Projects[0].Alias != "gamma" {
		t.Fatalf("expected reloaded *Env with alias gamma, got %+v", reloaded.Projects)
	}
}

func TestHTTPInvalidProjectsReturns500WithoutLeakingSecrets(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		EnvProjects: `[{"alias":"BAD","project_id":"proj1ab2c3d4","api_key":"sk_dummy_secret_in_env"}]`,
		EnvToken:    "dummy-mcp-token",
	}
	h := Handler(func(k string) string { return vars[k] })

	req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)))
	req.Header.Set("Authorization", "Bearer dummy-mcp-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("sk_dummy_secret_in_env")) {
		t.Fatalf("500 response leaked secret: %s", rec.Body.String())
	}
}
