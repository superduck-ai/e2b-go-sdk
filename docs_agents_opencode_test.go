package e2b_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsOpenCodeDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/opencode.mdx"); err != nil {
		t.Fatalf("agents opencode doc is missing: %v", err)
	}
}

// This test keeps docs/agents/opencode.mdx aligned with the exported Go SDK
// sandbox, git, HTTP, and template surface used to run OpenCode. The closures
// are compile-only examples and are intentionally never executed.
func TestDocsAgentsOpenCodeExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "run-headless",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "opencode", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `opencode run "Create a hello world HTTP server in Go"`, nil)
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

				sandbox, err := e2b.Create(ctx, "opencode", &e2b.SandboxOpts{
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
					`cd /home/user/repo && opencode run "Add error handling to all API endpoints"`,
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
			name: "run-http-server",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 10 * 60 * 1000
				client := &http.Client{Timeout: 10 * time.Second}

				sandbox, err := e2b.Create(ctx, "opencode", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout: "pause",
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				serverExecution, serverErr := sandbox.Commands.Run(ctx, "opencode serve --hostname 0.0.0.0 --port 4096", &e2b.CommandStartOpts{
					Background: true,
				})
				handle := serverExecution.(*e2b.CommandHandle)

				baseURL := "https://" + sandbox.GetHost(4096)

				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/global/health", nil)
				resp, doErr := client.Do(req)
				if resp != nil {
					resp.Body.Close()
				}

				sessionReq, sessionReqErr := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/session", nil)
				sessionResp, sessionErr := client.Do(sessionReq)
				var sessionBody []byte
				if sessionResp != nil {
					defer sessionResp.Body.Close()
					sessionBody, _ = io.ReadAll(sessionResp.Body)
				}

				var session map[string]any
				unmarshalSessionErr := json.Unmarshal(sessionBody, &session)
				sessionID, _ := session["id"].(string)

				payload, marshalErr := json.Marshal(map[string]any{
					"parts": []map[string]string{
						{
							"type": "text",
							"text": "Create a hello world HTTP server in Go",
						},
					},
				})
				messageReq, messageReqErr := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/session/"+sessionID+"/message", bytes.NewReader(payload))
				if messageReq != nil {
					messageReq.Header.Set("Content-Type", "application/json")
				}
				messageResp, messageErr := client.Do(messageReq)
				if messageResp != nil {
					defer messageResp.Body.Close()
					_, _ = io.ReadAll(messageResp.Body)
				}

				_ = handle
				_ = serverErr
				_ = reqErr
				_ = doErr
				_ = sessionReqErr
				_ = sessionErr
				_ = unmarshalSessionErr
				_ = marshalErr
				_ = messageReqErr
				_ = messageErr
			},
		},
		{
			name: "build-custom-template",
			fn: func() {
				template := e2b.Template(nil).
					FromTemplate("opencode").
					SetEnvs(map[string]string{
						"OPENCODE_SERVER_PASSWORD": "your-password",
					}).
					SetStartCmd(
						"opencode serve --hostname 0.0.0.0 --port 4096",
						e2b.WaitForPort(4096),
					)

				buildInfo, buildErr := e2b.Build(context.Background(), template, "my-opencode", &e2b.BuildOptions{
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

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 agents opencode doc snippets, got %d", got)
	}
}
