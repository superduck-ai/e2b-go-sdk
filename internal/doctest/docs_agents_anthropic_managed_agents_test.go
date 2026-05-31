package doctest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsAnthropicManagedAgentsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/anthropic-managed-agents.mdx"); err != nil {
		t.Fatalf("agents anthropic managed agents doc is missing: %v", err)
	}
}

// This test keeps docs/agents/anthropic-managed-agents.mdx aligned with the
// exported Go SDK sandbox, filesystem, command, connect, lifecycle, and host
// surface used for the E2B side of Anthropic Managed Agents. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsAgentsAnthropicManagedAgentsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "start-webhook-sandbox",
			fn: func() {
				ctx := context.Background()
				autoResume := true

				sandbox, err := e2b.Create(ctx, "E2B/claude-managed-agents-webhooks", &e2b.SandboxOpts{
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout:  "pause",
						AutoResume: &autoResume,
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeAPIKeyInfo, writeAPIKeyErr := sandbox.Files.Write(ctx, "/opt/anthropic-managed-agents-js/config/e2b-api-key", os.Getenv("E2B_API_KEY")+"\n", nil)
				writeAnthropicKeyInfo, writeAnthropicKeyErr := sandbox.Files.Write(ctx, "/opt/anthropic-managed-agents-js/config/anthropic-api-key", os.Getenv("ANTHROPIC_API_KEY")+"\n", nil)
				writeEnvironmentIDInfo, writeEnvironmentIDErr := sandbox.Files.Write(ctx, "/opt/anthropic-managed-agents-js/config/anthropic-environment-id", os.Getenv("ANTHROPIC_ENVIRONMENT_ID")+"\n", nil)
				writeEnvironmentKeyInfo, writeEnvironmentKeyErr := sandbox.Files.Write(ctx, "/opt/anthropic-managed-agents-js/config/anthropic-environment-key", os.Getenv("ANTHROPIC_ENVIRONMENT_KEY")+"\n", nil)
				webhookURL := "https://" + sandbox.GetHost(8000) + "/webhook"

				_ = writeAPIKeyInfo
				_ = writeAnthropicKeyInfo
				_ = writeEnvironmentIDInfo
				_ = writeEnvironmentKeyInfo
				_ = webhookURL
				_ = writeAPIKeyErr
				_ = writeAnthropicKeyErr
				_ = writeEnvironmentIDErr
				_ = writeEnvironmentKeyErr
			},
		},
		{
			name: "write-webhook-signing-key",
			fn: func() {
				ctx := context.Background()

				router, err := e2b.Connect(ctx, "sbx_router_123", nil)
				if err != nil {
					return
				}

				writeInfo, writeErr := router.Files.Write(
					ctx,
					"/opt/anthropic-managed-agents-js/config/anthropic-webhook-signing-key",
					os.Getenv("ANTHROPIC_WEBHOOK_SIGNING_KEY")+"\n",
					nil,
				)

				_ = writeInfo
				_ = writeErr
			},
		},
		{
			name: "inspect-router-health-and-assignments",
			fn: func() {
				ctx := context.Background()
				client := &http.Client{Timeout: 10 * time.Second}

				router, err := e2b.Connect(ctx, "sbx_router_123", nil)
				if err != nil {
					return
				}

				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+router.GetHost(8000)+"/health", nil)
				resp, doErr := client.Do(req)
				var body []byte
				if resp != nil {
					defer resp.Body.Close()
					body, _ = io.ReadAll(resp.Body)
				}

				var health map[string]any
				unmarshalHealthErr := json.Unmarshal(body, &health)

				logsExecution, logsErr := router.Commands.Run(ctx, "tail -200 /opt/anthropic-managed-agents-js/webhook.log || true", nil)
				assignmentsExecution, assignmentsErr := router.Commands.Run(ctx, "cat /opt/anthropic-managed-agents-js/.managed-agent-sandbox-store.json || true", nil)

				var assignments map[string]any
				unmarshalAssignmentsErr := json.Unmarshal([]byte(assignmentsExecution.(*e2b.CommandResult).Stdout), &assignments)

				_ = health["ok"]
				_ = logsExecution.(*e2b.CommandResult).Stdout
				_ = assignments
				_ = reqErr
				_ = doErr
				_ = unmarshalHealthErr
				_ = logsErr
				_ = assignmentsErr
				_ = unmarshalAssignmentsErr
			},
		},
		{
			name: "inspect-worker-sandbox",
			fn: func() {
				ctx := context.Background()

				worker, err := e2b.Connect(ctx, "sbx_worker_123", nil)
				if err != nil {
					return
				}

				execution, runErr := worker.Commands.Run(ctx, "tail -200 /opt/anthropic-managed-agents-js/worker.log || true", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 agents anthropic managed agents doc snippets, got %d", got)
	}
}
