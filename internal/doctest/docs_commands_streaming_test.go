package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsStreamingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands/streaming.mdx"); err != nil {
		t.Fatalf("commands streaming doc is missing: %v", err)
	}
}

func TestDocsCommandsStreamingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "stream-foreground-command",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello; sleep 1; echo world", &e2b.CommandStartOpts{
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})
				if !assert.NoError(t, runErr, "failed to run streaming command") {
					return
				}

				result := execution.(*e2b.CommandResult)
				_ = result.Stdout
				_ = result.Stderr
			},
		},
		{
			name: "stream-background-command",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect to sandbox") {
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
				if !assert.NoError(t, runErr, "failed to start background streaming command") {
					return
				}

				handle := execution.(*e2b.CommandHandle)
				state := handle.State()

				_ = state.Stdout
				_ = state.Stderr
				_, _ = handle.Kill()
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 commands streaming doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
