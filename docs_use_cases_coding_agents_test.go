package e2b_test

import (
	"context"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsUseCasesCodingAgentsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/use-cases/coding-agents.mdx"); err != nil {
		t.Fatalf("use-cases coding-agents doc is missing: %v", err)
	}
}

// This test keeps docs/use-cases/coding-agents.mdx aligned with the exported
// Go SDK workflow for running coding agents in a sandbox. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsUseCasesCodingAgentsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-agent",
			fn: func() {
				ctx := context.Background()
				timeoutMs := int((10 * time.Minute) / time.Millisecond)
				depth := 1

				sandbox, err := e2b.Create(ctx, "agent-template", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://github.com/org/repo.git", &e2b.GitCloneOpts{
					Path:  "/workspace",
					Depth: &depth,
				})

				execution, runErr := sandbox.Commands.Run(ctx, `cd /workspace && agent-binary --task "fix the failing tests"`, &e2b.CommandStartOpts{
					Background: true,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})

				handle := execution.(*e2b.CommandHandle)
				result, waitErr := handle.Wait()

				_ = cloneResult
				_ = cloneErr
				_ = result
				_ = runErr
				_ = waitErr
			},
		},
		{
			name: "collect-results",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				status, statusErr := sandbox.Git.Status(ctx, "/workspace", nil)
				diffExecution, diffErr := sandbox.Commands.Run(ctx, "cd /workspace && git diff --stat", nil)
				diff := diffExecution.(*e2b.CommandResult)

				artifactValue, artifactErr := sandbox.Files.Read(ctx, "/workspace/agent-output.json", nil)
				artifact := artifactValue.(string)

				_ = status.CurrentBranch
				_ = status.FileStatus
				_ = diff.Stdout
				_ = artifact
				_ = statusErr
				_ = diffErr
				_ = artifactErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 use-cases coding-agents doc snippets, got %d", got)
	}
}
