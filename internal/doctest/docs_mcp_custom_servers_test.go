package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpCustomServersDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp/custom-servers.mdx"); err != nil {
		t.Fatalf("mcp custom-servers doc is missing: %v", err)
	}
}

// This test keeps docs/mcp/custom-servers.mdx aligned with the exported Go SDK
// MCP custom server surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsMcpCustomServersExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "custom-server-config",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Mcp: e2b.McpServer{
						"github/modelcontextprotocol/servers": map[string]any{
							"installCmd": "npm install",
							"runCmd":     "sudo npx -y @modelcontextprotocol/server-filesystem /root",
						},
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				mcpURL := sandbox.GetMcpUrl()
				mcpToken, tokenErr := sandbox.GetMcpToken()

				_ = mcpURL
				_ = mcpToken
				_ = tokenErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 mcp custom-servers doc snippet, got %d", got)
	}
}
