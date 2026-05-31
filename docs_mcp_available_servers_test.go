package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpAvailableServersDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp/available-servers.mdx"); err != nil {
		t.Fatalf("mcp available servers doc is missing: %v", err)
	}
}

// This test keeps docs/mcp/available-servers.mdx aligned with the exported Go
// SDK MCP catalog configuration surface. The closures are compile-only examples
// and are intentionally never executed.
func TestDocsMcpAvailableServersExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "runtime-catalog-servers",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "", &e2b.SandboxOpts{
					Mcp: e2b.McpServer{
						"browserbase": map[string]any{
							"apiKey":       "browserbase-api-key",
							"geminiApiKey": "gemini-api-key",
							"projectId":    "project-id",
						},
						"airtable": map[string]any{
							"airtableApiKey": "airtable-api-key",
						},
						"brave": map[string]any{
							"apiKey": "brave-api-key",
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
			name: "template-prepull-catalog-servers",
			fn: func() {
				template := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("exa", []string{"brave", "browserbase"})

				_ = template
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 mcp available servers doc snippets, got %d", got)
	}
}
