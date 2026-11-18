package server

import (
	"crypto/sha256"
	"log"
	"net/http"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat-org/internal/httpclient"
)

// Handler serves GET /health and the MCP endpoint at /mcp. getenv is called
// per request so the Cloudflare Worker can pass cloudflare.Getenv.
func Handler(getenv func(string) string) http.Handler {
	mcpHandler := mcp.NewStreamableHTTPHandler(serverFrom, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// Access is gated by MCP_TOKEN / MCP_READ_TOKEN, and deployments sit behind proxies forwarding a public Host.
		DisableLocalhostProtection: true,
	})

	envs := &envCache{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok\n"))
	})
	mux.Handle("/mcp", requireToken(getenv, withEnv(getenv, envs, mcpHandler)))

	return mux
}

func withEnv(getenv func(string) string, envs *envCache, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env, err := envs.get(getenv)
		if err != nil {
			log.Printf("server: %v", err)
			http.Error(w, "server is not configured", http.StatusInternalServerError)
			return
		}

		next.ServeHTTP(w, r.WithContext(WithEnv(r.Context(), env)))
	})
}

// envCache reuses the parsed Env while REVENUECAT_PROJECTS stays unchanged,
// hashing the raw JSON so secret keys are not duplicated in cache state.
type envCache struct {
	mu   sync.Mutex
	hash [32]byte
	env  *Env
}

func (c *envCache) get(getenv func(string) string) (*Env, error) {
	projectsRaw := getenv(EnvProjects)
	sum := sha256.Sum256([]byte(projectsRaw))

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.env != nil && c.hash == sum {
		return c.env, nil
	}

	env, err := NewEnv(projectsRaw, httpclient.New())
	if err != nil {
		return nil, err
	}

	c.hash, c.env = sum, env

	return env, nil
}
