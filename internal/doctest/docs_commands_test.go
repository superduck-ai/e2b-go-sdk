package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands.mdx"); err != nil {
		t.Fatalf("commands overview doc is missing: %v", err)
	}
}

// This test keeps docs/commands.mdx aligned with the exported Go SDK commands
// overview surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCommandsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-foreground-command",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "ls -l", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.ExitCode
				_ = result.Stdout
				_ = result.Stderr
				_ = result.Error
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 commands overview doc snippet, got %d", got)
	}
}
