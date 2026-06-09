package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands.mdx"); err != nil {
		t.Fatalf("commands overview doc is missing: %v", err)
	}
}

func TestDocsCommandsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "run-foreground-command",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "ls -l", nil)
				if !assert.NoError(t, runErr, "failed to run command") {
					return
				}
				result := execution.(*e2b.CommandResult)

				_ = result.ExitCode
				_ = result.Stdout
				_ = result.Stderr
				_ = result.Error
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 commands overview doc snippet, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
