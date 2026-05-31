package e2b_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsOpenClawGatewayDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/openclaw/openclaw-gateway.mdx"); err != nil {
		t.Fatalf("agents openclaw gateway doc is missing: %v", err)
	}
}

// This test keeps docs/agents/openclaw/openclaw-gateway.mdx aligned with the
// exported Go SDK sandbox, command, lifecycle, and host surface used to run
// the OpenClaw gateway. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsAgentsOpenClawGatewayExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "quick-start",
			fn: func() {
				ctx := context.Background()
				port := 18789
				token := os.Getenv("OPENCLAW_APP_TOKEN")
				if token == "" {
					token = "my-gateway-token"
				}
				timeoutMs := 3600_000

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
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

				_, modelErr := sandbox.Commands.Run(ctx, "openclaw config set agents.defaults.model.primary openai/gpt-5.2", nil)
				_, insecureErr := sandbox.Commands.Run(ctx, "openclaw config set gateway.controlUi.allowInsecureAuth true", nil)
				_, deviceErr := sandbox.Commands.Run(ctx, "openclaw config set gateway.controlUi.dangerouslyDisableDeviceAuth true", nil)

				startCmd := fmt.Sprintf(
					"openclaw gateway --allow-unconfigured --bind lan --auth token --token %s --port %d",
					token,
					port,
				)
				execution, runErr := sandbox.Commands.Run(ctx, startCmd, &e2b.CommandStartOpts{
					Background: true,
				})
				gateway := execution.(*e2b.CommandHandle)

				probeCmd := fmt.Sprintf(`bash -lc 'ss -ltn | grep -q ":%d " && echo ready || echo waiting'`, port)
				probeExecution, probeErr := sandbox.Commands.Run(ctx, probeCmd, nil)
				probe := probeExecution.(*e2b.CommandResult)
				url := fmt.Sprintf("https://%s/?token=%s", sandbox.GetHost(port), token)

				_ = gateway
				_ = probe.Stdout
				_ = url
				_ = modelErr
				_ = insecureErr
				_ = deviceErr
				_ = runErr
				_ = probeErr
			},
		},
		{
			name: "approve-browser-device",
			fn: func() {
				ctx := context.Background()
				port := 18789
				token := os.Getenv("OPENCLAW_APP_TOKEN")
				if token == "" {
					token = "my-gateway-token"
				}

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				listCmd := fmt.Sprintf(
					"openclaw devices list --json --url ws://127.0.0.1:%d --token %s",
					port,
					token,
				)
				listExecution, listErr := sandbox.Commands.Run(ctx, listCmd, nil)
				list := listExecution.(*e2b.CommandResult)

				var payload struct {
					Pending []struct {
						RequestID string `json:"requestId"`
					} `json:"pending"`
				}
				unmarshalErr := json.Unmarshal([]byte(list.Stdout), &payload)

				var approveResult any
				var approveErr error
				if len(payload.Pending) > 0 {
					approveCmd := fmt.Sprintf(
						"openclaw devices approve %s --token %s --url ws://127.0.0.1:%d",
						payload.Pending[0].RequestID,
						token,
						port,
					)
					approveResult, approveErr = sandbox.Commands.Run(ctx, approveCmd, nil)
				}

				_ = approveResult
				_ = listErr
				_ = unmarshalErr
				_ = approveErr
			},
		},
		{
			name: "restart-gateway",
			fn: func() {
				ctx := context.Background()
				port := 18789
				token := os.Getenv("OPENCLAW_APP_TOKEN")
				if token == "" {
					token = "my-gateway-token"
				}

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				killCmd := `bash -lc 'for p in "[o]penclaw gateway" "[o]penclaw-gateway"; do for pid in $(pgrep -f "$p" || true); do kill "$pid" >/dev/null 2>&1 || true; done; done'`
				_, killErr := sandbox.Commands.Run(ctx, killCmd, nil)

				startCmd := fmt.Sprintf(
					"openclaw gateway --allow-unconfigured --bind lan --auth token --token %s --port %d",
					token,
					port,
				)
				execution, runErr := sandbox.Commands.Run(ctx, startCmd, &e2b.CommandStartOpts{
					Background: true,
				})
				gateway := execution.(*e2b.CommandHandle)

				probeCmd := fmt.Sprintf(`bash -lc 'ss -ltn | grep -q ":%d " && echo ready || echo waiting'`, port)
				probeExecution, probeErr := sandbox.Commands.Run(ctx, probeCmd, nil)
				probe := probeExecution.(*e2b.CommandResult)
				ready := strings.TrimSpace(probe.Stdout) == "ready"
				_ = time.Second

				_ = gateway
				_ = ready
				_ = killErr
				_ = runErr
				_ = probeErr
			},
		},
		{
			name: "turn-insecure-flags-off",
			fn: func() {
				ctx := context.Background()
				port := 18789
				token := os.Getenv("OPENCLAW_APP_TOKEN")
				if token == "" {
					token = "my-gateway-token"
				}

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY": os.Getenv("OPENAI_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, insecureErr := sandbox.Commands.Run(ctx, "openclaw config set gateway.controlUi.allowInsecureAuth false", nil)
				_, deviceErr := sandbox.Commands.Run(ctx, "openclaw config set gateway.controlUi.dangerouslyDisableDeviceAuth false", nil)

				killCmd := `bash -lc 'for p in "[o]penclaw gateway" "[o]penclaw-gateway"; do for pid in $(pgrep -f "$p" || true); do kill "$pid" >/dev/null 2>&1 || true; done; done'`
				_, killErr := sandbox.Commands.Run(ctx, killCmd, nil)

				startCmd := fmt.Sprintf(
					"openclaw gateway --allow-unconfigured --bind lan --auth token --token %s --port %d",
					token,
					port,
				)
				execution, runErr := sandbox.Commands.Run(ctx, startCmd, &e2b.CommandStartOpts{
					Background: true,
				})
				gateway := execution.(*e2b.CommandHandle)

				_ = gateway
				_ = insecureErr
				_ = deviceErr
				_ = killErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 agents openclaw gateway doc snippets, got %d", got)
	}
}
