package doctest

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsAmpDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/amp.mdx"); err != nil {
		t.Fatalf("agents amp doc is missing: %v", err)
	}
}

// This test keeps docs/agents/amp.mdx aligned with the exported Go SDK
// sandbox, git, command, and template surface used to run Amp. The closures
// are compile-only examples and are intentionally never executed.
func TestDocsAgentsAmpExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-headless",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "amp", &e2b.SandboxOpts{
					Envs: map[string]string{
						"AMP_API_KEY": os.Getenv("AMP_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`amp --dangerously-allow-all -x "Create a hello world HTTP server in Go"`,
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

				sandbox, err := e2b.Create(ctx, "amp", &e2b.SandboxOpts{
					Envs: map[string]string{
						"AMP_API_KEY": os.Getenv("AMP_API_KEY"),
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
					`cd /home/user/repo && amp --dangerously-allow-all -x "Add error handling to all API endpoints"`,
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
			name: "streaming-json",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "amp", &e2b.SandboxOpts{
					Envs: map[string]string{
						"AMP_API_KEY": os.Getenv("AMP_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, runErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && amp --dangerously-allow-all --stream-json -x "Find and fix all TODO comments"`,
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
			name: "thread-management",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 600_000

				sandbox, err := e2b.Create(ctx, "amp", &e2b.SandboxOpts{
					Envs: map[string]string{
						"AMP_API_KEY": os.Getenv("AMP_API_KEY"),
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, initialErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && amp --dangerously-allow-all -x "Analyze the codebase and create a refactoring plan"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				threadsExecution, threadsErr := sandbox.Commands.Run(ctx, "amp threads list --json", nil)
				threads := threadsExecution.(*e2b.CommandResult)

				var threadList []map[string]any
				unmarshalErr := json.Unmarshal([]byte(threads.Stdout), &threadList)
				threadID, _ := threadList[0]["id"].(string)

				_, followErr := sandbox.Commands.Run(
					ctx,
					`cd /home/user/repo && amp threads continue `+threadID+` --dangerously-allow-all -x "Now implement step 1 of the plan"`,
					&e2b.CommandStartOpts{
						OnStdout: func(data e2b.Stdout) {
							_ = data
						},
					},
				)

				_ = initialErr
				_ = threadsErr
				_ = unmarshalErr
				_ = followErr
			},
		},
		{
			name: "build-custom-template",
			fn: func() {
				template := e2b.Template(nil).FromTemplate("amp")

				buildInfo, buildErr := e2b.Build(context.Background(), template, "my-amp", &e2b.BuildOptions{
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

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 agents amp doc snippets, got %d", got)
	}
}
