package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsQuickstartDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/quickstart.mdx"); err != nil {
		t.Fatalf("quickstart doc is missing: %v", err)
	}
}

// This test keeps docs/quickstart.mdx aligned with the exported Go SDK
// sandbox quickstart surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsQuickstartExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "first-sandbox",
			fn: func() {
				_ = godotenv.Load()
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `python3 -c "print('hello world')"`, nil)
				result := execution.(*e2b.CommandResult)

				entries, listErr := sandbox.Files.List(ctx, "/", nil)

				_ = result.Stdout
				_ = entries
				_ = runErr
				_ = listErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 quickstart doc snippet, got %d", got)
	}
}
