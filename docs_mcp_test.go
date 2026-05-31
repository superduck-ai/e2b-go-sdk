package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp.mdx"); err != nil {
		t.Fatalf("mcp overview doc is missing: %v", err)
	}
}

// This test keeps docs/mcp.mdx aligned with the exported Go SDK MCP overview
// surface. The closures are compile-only examples and are intentionally never
// executed.
func TestDocsMcpOverviewCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "start-sandbox-with-mcp",
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
						"airtable": map[string]any{
							"airtableApiKey": "airtable-api-key",
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
			name: "connect-mcp-client",
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

				mcpURL := sandbox.GetMcpUrl()
				mcpToken, tokenErr := sandbox.GetMcpToken()

				_ = mcpURL
				_ = mcpToken
				_ = tokenErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 mcp overview doc snippets, got %d", got)
	}
}
