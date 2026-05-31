package e2b_test

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsClaudeCodeDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/claude-code.mdx"); err != nil {
		t.Fatalf("agents claude-code doc is missing: %v", err)
	}
}

// This test keeps docs/agents/claude-code.mdx aligned with the exported Go SDK
// sandbox, git, filesystem, MCP, and template surface used to run Claude Code.
// The closures are compile-only examples and are intentionally never executed.
func TestDocsAgentsClaudeCodeExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-headless",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
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
					`claude --dangerously-skip-permissions -p "Create a hello world HTTP server in Go"`,
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

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
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
					`cd /home/user/repo && claude --dangerously-skip-permissions -p "Add error handling to all API endpoints"`,
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

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
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
					`claude --dangerously-skip-permissions --output-format json -p "Review this codebase and list all security issues as JSON"`,
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
			name: "streaming-output",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, runErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && claude --dangerously-skip-permissions --output-format stream-json -p "Find and fix all TODO comments"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							for _, line := range strings.Split(string(data), "\n") {
								if strings.TrimSpace(line) == "" {
									continue
								}
								var event map[string]any
								_ = json.Unmarshal([]byte(line), &event)
								_ = event["type"]
							}
						},
					},
				)

				_ = runErr
			},
		},
		{
			name: "resume-session",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 600_000

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				initialExecution, initialErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && claude --dangerously-skip-permissions --output-format json -p "Analyze the codebase and create a refactoring plan"`,
					nil,
				)
				initial := initialExecution.(*e2b.CommandResult)

				var response map[string]any
				unmarshalErr := json.Unmarshal([]byte(initial.Stdout), &response)
				sessionID, _ := response["session_id"].(string)

				_, followErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && claude --dangerously-skip-permissions --resume `+sessionID+` -p "Now implement step 1 of the plan"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				_ = initialErr
				_ = unmarshalErr
				_ = followErr
			},
		},
		{
			name: "custom-system-prompt",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/repo/CLAUDE.md", `
You are working on a Go microservice.
Always use structured logging with slog.
Follow the project's error handling conventions in pkg/errors.
`, nil)
				execution, runErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && claude --dangerously-skip-permissions -p "Add a /healthz endpoint"`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				_ = writeInfo
				_ = result.Stdout
				_ = writeErr
				_ = runErr
			},
		},
		{
			name: "connect-mcp-tools",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "claude", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
					Mcp: e2b.McpServer{
						"browserbase": map[string]any{
							"apiKey":    os.Getenv("BROWSERBASE_API_KEY"),
							"projectId": os.Getenv("BROWSERBASE_PROJECT_ID"),
						},
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				mcpURL := sandbox.GetMcpUrl()
				mcpToken, tokenErr := sandbox.GetMcpToken()

				_, addErr := sandbox.Commands.Run(
					ctx,
					`claude mcp add --transport http e2b-mcp-gateway `+mcpURL+` --header "Authorization: Bearer `+mcpToken+`"`,
					nil,
				)
				_, runErr := sandbox.Commands.Run(
					ctx,
					`claude --dangerously-skip-permissions -p "Use browserbase to research E2B and summarize your findings"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				_ = tokenErr
				_ = addErr
				_ = runErr
			},
		},
		{
			name: "build-custom-template",
			fn: func() {
				template := e2b.Template(nil).FromTemplate("claude")

				buildInfo, buildErr := e2b.Build(context.Background(), template, "my-claude", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
				if buildErr != nil {
					log.Print(buildErr)
				}

				_ = buildInfo
			},
		},
	}

	if got := len(snippets); got != 8 {
		t.Fatalf("expected 8 agents claude-code doc snippets, got %d", got)
	}
}
