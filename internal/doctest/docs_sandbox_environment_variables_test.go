package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxEnvironmentVariablesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/environment-variables.mdx"); err != nil {
		t.Fatalf("sandbox environment variables doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/environment-variables.mdx aligned with the
// exported Go SDK environment variable surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxEnvironmentVariablesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "runtime-env-vars",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "printenv E2B_SANDBOX_ID", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "sandbox-wide-envs",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Envs: map[string]string{
						"MY_VAR": "my_value",
					},
				})
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "printenv MY_VAR", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "command-scoped-envs",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "printenv MY_VAR", &e2b.CommandStartOpts{
					Envs: map[string]string{
						"MY_VAR": "123",
					},
				})
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 sandbox environment variables doc snippets, got %d", got)
	}
}
