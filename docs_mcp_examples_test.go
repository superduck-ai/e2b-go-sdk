package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpExamplesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp/examples.mdx"); err != nil {
		t.Fatalf("mcp examples doc is missing: %v", err)
	}
}

// This test keeps docs/mcp/examples.mdx aligned with the exported Go SDK MCP
// example patterns. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsMcpExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "runtime-gateway",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Mcp: e2b.McpServer{
						"exa": map[string]any{
							"apiKey": "exa-api-key",
						},
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				mcpToken, tokenErr := sandbox.GetMcpToken()
				mcpURL := sandbox.GetMcpUrl()

				_ = mcpToken
				_ = tokenErr
				_ = mcpURL
			},
		},
		{
			name: "prepulled-template",
			fn: func() {
				template := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("exa", "browserbase")

				_ = template
			},
		},
		{
			name: "custom-server-entry",
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
				if sandbox != nil {
					defer sandbox.Kill(context.Background(), nil)
				}

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 mcp examples doc snippets, got %d", got)
	}
}
