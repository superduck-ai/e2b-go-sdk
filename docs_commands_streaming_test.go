package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsStreamingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands/streaming.mdx"); err != nil {
		t.Fatalf("commands streaming doc is missing: %v", err)
	}
}

// This test keeps docs/commands/streaming.mdx aligned with the exported Go SDK
// streaming-command surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCommandsStreamingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "stream-foreground-command",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello; sleep 1; echo world", &e2b.CommandStartOpts{
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})

				result := execution.(*e2b.CommandResult)
				_ = result.Stdout
				_ = result.Stderr
				_ = runErr
			},
		},
		{
			name: "stream-background-command",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello; sleep 10; echo world", &e2b.CommandStartOpts{
					Background: true,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})

				handle := execution.(*e2b.CommandHandle)
				state := handle.State()

				_ = state.Stdout
				_ = state.Stderr
				_, _ = handle.Kill()
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 commands streaming doc snippets, got %d", got)
	}
}
