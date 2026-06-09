package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsQuickstartDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/quickstart.mdx"); err != nil {
		t.Fatalf("quickstart doc is missing: %v", err)
	}
}

func TestDocsQuickstartExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "first-sandbox",
			fn: func(t *testing.T) {
				_ = godotenv.Load()
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `python3 -c "print('hello world')"`, nil)
				if !assert.NoError(t, runErr, "failed to run python") {
					return
				}
				result := execution.(*e2b.CommandResult)

				entries, listErr := sandbox.Files.List(ctx, "/", nil)
				if !assert.NoError(t, listErr, "failed to list /") {
					return
				}

				_ = result.Stdout
				_ = entries
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 quickstart doc snippet, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
