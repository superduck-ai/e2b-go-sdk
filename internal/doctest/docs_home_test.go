package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsHomeDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs.mdx"); err != nil {
		t.Fatalf("docs home page is missing: %v", err)
	}
}

// This test keeps docs.mdx aligned with the exported Go SDK sandbox and
// commands surface used on the documentation home page. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsHomeExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-sandbox-and-run-command",
			fn: func() {

				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}

				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `echo "Hello from E2B Sandbox!"`, nil)
				if !assert.NoError(t, runErr, "failed to run command") {
					return
				}
				result := execution.(*e2b.CommandResult)

				t.Log("result:", result.Stdout)
				assert.Equal(t, "Hello from E2B Sandbox!\n", result.Stdout, "unexpected command output")
				_ = result.Stdout
			},
		},
	}
	for _, snippet := range snippets {
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn()
		})
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 docs home page snippet, got %d", got)
	}
}
