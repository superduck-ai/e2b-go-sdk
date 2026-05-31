package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsDevinDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/devin.mdx"); err != nil {
		t.Fatalf("agents devin doc is missing: %v", err)
	}
}

// This test keeps docs/agents/devin.mdx aligned with the exported Go SDK
// sandbox, PTY, connect, git, and command surface used to run Devin. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsAgentsDevinExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "install-and-login-with-pty",
			fn: func() {
				ctx := context.Background()
				sandboxTimeoutMs := 3600_000
				ptyTimeoutMs := 0

				sandbox, err := e2b.Create(ctx, "base", &e2b.SandboxOpts{
					TimeoutMs: &sandboxTimeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout: "pause",
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      120,
					Rows:      36,
					TimeoutMs: &ptyTimeoutMs,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if ptyErr != nil {
					return
				}

				installErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("curl -fsSL https://cli.devin.ai/install.sh | bash\n"), nil)
				sourceErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("source /home/user/.bashrc\n"), nil)
				loginErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("devin login\n"), nil)
				terminalKilled, terminalKillErr := terminal.Kill()

				_ = sandbox.SandboxID
				_ = installErr
				_ = sourceErr
				_ = loginErr
				_ = terminalKilled
				_ = terminalKillErr
			},
		},
		{
			name: "run-noninteractive-prompt",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`mkdir -p /home/user/project && cd /home/user/project && devin --permission-mode dangerous -p "Create a hello world HTTP server in Go"`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "work-on-cloned-repository",
			fn: func() {
				ctx := context.Background()
				depth := 1

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://github.com/your-org/your-repo.git", &e2b.GitCloneOpts{
					Path:     "/home/user/repo",
					Username: "x-access-token",
					Password: os.Getenv("GITHUB_TOKEN"),
					Depth:    &depth,
				})

				_, runErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && devin --permission-mode dangerous -p "Add error handling to all API endpoints"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				diffExecution, diffErr := sandbox.Commands.Run(ctx, "cd /home/user/repo && git diff", nil)
				diff := diffExecution.(*e2b.CommandResult)

				_ = cloneResult
				_ = cloneErr
				_ = diff.Stdout
				_ = runErr
				_ = diffErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 agents devin doc snippets, got %d", got)
	}
}
