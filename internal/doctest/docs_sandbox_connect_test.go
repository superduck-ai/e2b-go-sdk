package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxConnectDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/connect.mdx"); err != nil {
		t.Fatalf("sandbox connect doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/connect.mdx aligned with the exported Go SDK
// connect and list surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxConnectExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "find-sandbox-id",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						State: []e2b.SandboxState{e2b.SandboxState("running")},
					},
				})

				runningSandboxes, err := paginator.NextItems()
				var sandboxID string
				if len(runningSandboxes) > 0 {
					sandboxID = runningSandboxes[0].SandboxID
				}

				_ = sandboxID
				_ = err
			},
		},
		{
			name: "connect-and-run-command",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "whoami", nil)
				result := execution.(*e2b.CommandResult)

				_ = sandbox.SandboxID
				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "connect-with-timeout",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 60_000

				sandbox, err := e2b.Connect(ctx, "sbx_123", &e2b.SandboxConnectOpts{
					TimeoutMs: &timeoutMs,
				})

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 sandbox connect doc snippets, got %d", got)
	}
}
