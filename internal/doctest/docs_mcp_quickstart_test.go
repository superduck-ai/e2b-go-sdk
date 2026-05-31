package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpQuickstartDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp/quickstart.mdx"); err != nil {
		t.Fatalf("mcp quickstart doc is missing: %v", err)
	}
}

// This test keeps docs/mcp/quickstart.mdx aligned with the exported Go SDK MCP
// quickstart surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsMcpQuickstartExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "outside-the-sandbox",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Mcp: e2b.McpServer{
						"browserbase": map[string]any{
							"apiKey":       "browserbase-api-key",
							"geminiApiKey": "gemini-api-key",
							"projectId":    "project-id",
						},
						"exa": map[string]any{
							"apiKey": "exa-api-key",
						},
						"notion": map[string]any{
							"internalIntegrationToken": "notion-api-key",
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
		{
			name: "inside-the-sandbox",
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
				execution, runErr := sandbox.Commands.Run(ctx, "echo http://localhost:50005/mcp", nil)
				result := execution.(*e2b.CommandResult)

				_ = mcpToken
				_ = tokenErr
				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 mcp quickstart doc snippets, got %d", got)
	}
}
