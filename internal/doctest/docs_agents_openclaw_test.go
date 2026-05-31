package doctest

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsOpenClawDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/openclaw.mdx"); err != nil {
		t.Fatalf("agents openclaw doc is missing: %v", err)
	}
}

// This test keeps docs/agents/openclaw.mdx aligned with the exported Go SDK
// sandbox, git, filesystem, command, and template surface used to run
// OpenClaw. The closures are compile-only examples and are intentionally never
// executed.
func TestDocsAgentsOpenClawExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-headless",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`openclaw agent --local --thinking high --message "Create a hello world HTTP server in Go"`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "work-on-cloned-repo",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 600_000
				depth := 1

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				cloneResult, cloneErr := sandbox.Git.Clone(ctx, "https://github.com/your-org/your-repo.git", &e2b.GitCloneOpts{
					Path:     "/home/user/repo",
					Username: "x-access-token",
					Password: os.Getenv("GITHUB_TOKEN"),
					Depth:    &depth,
				})

				_, runErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && openclaw agent --local --thinking high --message "Add error handling to all API endpoints"`,
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
		{
			name: "structured-output",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`openclaw agent --local --json --message "List all files in the current directory and describe each"`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				var response map[string]any
				unmarshalErr := json.Unmarshal([]byte(result.Stdout), &response)

				_ = response
				_ = runErr
				_ = unmarshalErr
			},
		},
		{
			name: "customize-soul",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				soul := `# Soul

## Core Truths
- Be genuinely helpful.
- Read the workspace before asking for more context.

## Boundaries
- Do not leak user data.
- Be cautious with destructive actions.
`

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/.openclaw/workspaces/default/SOUL.md", soul, nil)
				_, runErr := sandbox.Commands.Run(
					ctx,
					`openclaw agent --local --thinking high --message "Introduce yourself"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				_ = writeInfo
				_ = writeErr
				_ = runErr
			},
		},
		{
			name: "customize-heartbeat",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				heartbeat := `# Heartbeat Checklist

- Check for new issues in the repository.
- Run tests if files changed.
- Review stale pull requests.
`

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/.openclaw/workspaces/default/HEARTBEAT.md", heartbeat, nil)
				_, runErr := sandbox.Commands.Run(
					ctx,
					`openclaw agent --local --message "Start monitoring the project"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				_ = writeInfo
				_ = writeErr
				_ = runErr
			},
		},
		{
			name: "build-custom-template",
			fn: func() {
				template := e2b.Template(nil).
					FromTemplate("openclaw").
					SetStartCmd(
						"openclaw gateway --port 18789 --verbose",
						e2b.WaitForPort(18789),
					)

				buildInfo, buildErr := e2b.Build(context.Background(), template, "my-openclaw", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 agents openclaw doc snippets, got %d", got)
	}
}
