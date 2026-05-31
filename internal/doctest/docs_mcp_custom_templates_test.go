package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsMcpCustomTemplatesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/mcp/custom-templates.mdx"); err != nil {
		t.Fatalf("mcp custom-templates doc is missing: %v", err)
	}
}

// This test keeps docs/mcp/custom-templates.mdx aligned with the exported Go
// SDK MCP template helpers. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsMcpCustomTemplatesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-mcp-template",
			fn: func() {
				_ = godotenv.Load()

				template := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("browserbase", []string{"exa"})

				buildInfo, err := e2b.Build(context.Background(), template, "my-mcp-gateway", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    8,
						MemoryMB:    8192,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = err
			},
		},
		{
			name: "add-multiple-mcp-servers",
			fn: func() {
				template := e2b.Template(nil).
					FromTemplate("mcp-gateway").
					AddMcpServer("browserbase", "exa", "notion")

				_ = template
			},
		},
		{
			name: "use-mcp-template",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "my-mcp-gateway", &e2b.SandboxOpts{
					Mcp: e2b.McpServer{
						"browserbase": map[string]any{
							"apiKey":       "browserbase-api-key",
							"geminiApiKey": "gemini-api-key",
							"projectId":    "project-id",
						},
						"exa": map[string]any{
							"apiKey": "exa-api-key",
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
		t.Fatalf("expected 3 mcp custom-templates doc snippets, got %d", got)
	}
}
