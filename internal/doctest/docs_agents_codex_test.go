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

func TestDocsAgentsCodexDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/codex.mdx"); err != nil {
		t.Fatalf("agents codex doc is missing: %v", err)
	}
}

// This test keeps docs/agents/codex.mdx aligned with the exported Go SDK
// sandbox, git, filesystem, and template surface used to run Codex. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsAgentsCodexExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-headless",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "codex", &e2b.SandboxOpts{
					Envs: map[string]string{
						"CODEX_API_KEY": os.Getenv("CODEX_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`codex exec --full-auto --skip-git-repo-check "Create a hello world HTTP server in Go"`,
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

				sandbox, err := e2b.Create(ctx, "codex", &e2b.SandboxOpts{
					Envs: map[string]string{
						"CODEX_API_KEY": os.Getenv("CODEX_API_KEY"),
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
					`codex exec --full-auto --skip-git-repo-check -C /home/user/repo "Add error handling to all API endpoints"`,
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
			name: "schema-validated-output",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "codex", &e2b.SandboxOpts{
					Envs: map[string]string{
						"CODEX_API_KEY": os.Getenv("CODEX_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				schema := `{
  "type": "object",
  "properties": {
    "issues": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "file": {"type": "string"},
          "line": {"type": "number"},
          "severity": {"type": "string"},
          "description": {"type": "string"}
        },
        "required": ["file", "severity", "description"]
      }
    }
  },
  "required": ["issues"]
}`

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/schema.json", schema, nil)
				execution, runErr := sandbox.Commands.Run(
					ctx,
					`codex exec --full-auto --skip-git-repo-check --output-schema /home/user/schema.json -C /home/user/repo "Review this codebase for security issues"`,
					nil,
				)
				result := execution.(*e2b.CommandResult)

				var response map[string]any
				unmarshalErr := json.Unmarshal([]byte(result.Stdout), &response)

				_ = writeInfo
				_ = response["issues"]
				_ = writeErr
				_ = runErr
				_ = unmarshalErr
			},
		},
		{
			name: "streaming-events",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "codex", &e2b.SandboxOpts{
					Envs: map[string]string{
						"CODEX_API_KEY": os.Getenv("CODEX_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, runErr := sandbox.Commands.Run(
					ctx,
					`codex exec --full-auto --skip-git-repo-check --json -C /home/user/repo "Refactor the utils module into separate files"`,
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
			name: "image-input",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 600_000

				sandbox, err := e2b.Create(ctx, "codex", &e2b.SandboxOpts{
					Envs: map[string]string{
						"CODEX_API_KEY": os.Getenv("CODEX_API_KEY"),
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/home/user/mockup.png", []byte("image-bytes"), nil)
				_, runErr := sandbox.Commands.Run(
					ctx,
					`codex exec --full-auto --skip-git-repo-check --image /home/user/mockup.png -C /home/user/repo "Implement this UI design as a React component"`,
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
				template := e2b.Template(nil).FromTemplate("codex")

				buildInfo, buildErr := e2b.Build(context.Background(), template, "my-codex", &e2b.BuildOptions{
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

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 agents codex doc snippets, got %d", got)
	}
}
