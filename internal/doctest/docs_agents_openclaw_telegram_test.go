package doctest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAgentsOpenClawTelegramDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/openclaw/openclaw-telegram.mdx"); err != nil {
		t.Fatalf("agents openclaw telegram doc is missing: %v", err)
	}
}

// This test keeps docs/agents/openclaw/openclaw-telegram.mdx aligned with the
// exported Go SDK sandbox and command surface used to run Telegram-backed
// OpenClaw flows. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsAgentsOpenClawTelegramExamplesCompile(t *testing.T) {
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
					token = "my-openclaw-token"
				}
				timeoutMs := 3600_000

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY":     os.Getenv("OPENAI_API_KEY"),
						"TELEGRAM_BOT_TOKEN": os.Getenv("TELEGRAM_BOT_TOKEN"),
					},
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, modelErr := sandbox.Commands.Run(ctx, "openclaw config set agents.defaults.model.primary openai/gpt-5.2", nil)
				_, pluginErr := sandbox.Commands.Run(ctx, "openclaw config set plugins.entries.telegram.enabled true", nil)

				addChannelCmd := fmt.Sprintf(
					"openclaw channels add --channel telegram --token %s",
					os.Getenv("TELEGRAM_BOT_TOKEN"),
				)
				_, addErr := sandbox.Commands.Run(ctx, addChannelCmd, nil)
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
				ready := strings.TrimSpace(probe.Stdout) == "ready"

				_ = gateway
				_ = ready
				_ = modelErr
				_ = pluginErr
				_ = addErr
				_ = insecureErr
				_ = deviceErr
				_ = runErr
				_ = probeErr
			},
		},
		{
			name: "approve-pairing-code",
			fn: func() {
				ctx := context.Background()
				pairingCode := "XXXXXXXX"

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY":     os.Getenv("OPENAI_API_KEY"),
						"TELEGRAM_BOT_TOKEN": os.Getenv("TELEGRAM_BOT_TOKEN"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				approveCmd := fmt.Sprintf(
					"openclaw pairing approve --channel telegram %s",
					pairingCode,
				)
				approveResult, approveErr := sandbox.Commands.Run(ctx, approveCmd, nil)

				_ = approveResult
				_ = approveErr
			},
		},
		{
			name: "verify-channel-status",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY":     os.Getenv("OPENAI_API_KEY"),
						"TELEGRAM_BOT_TOKEN": os.Getenv("TELEGRAM_BOT_TOKEN"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				channelsExecution, channelsErr := sandbox.Commands.Run(ctx, "openclaw channels list --json", nil)
				statusExecution, statusErr := sandbox.Commands.Run(ctx, "openclaw channels status --json --probe", nil)
				pairingExecution, pairingErr := sandbox.Commands.Run(ctx, "openclaw pairing list --json --channel telegram", nil)

				var channels any
				unmarshalChannelsErr := json.Unmarshal([]byte(channelsExecution.(*e2b.CommandResult).Stdout), &channels)
				var status any
				unmarshalStatusErr := json.Unmarshal([]byte(statusExecution.(*e2b.CommandResult).Stdout), &status)
				var pairing any
				unmarshalPairingErr := json.Unmarshal([]byte(pairingExecution.(*e2b.CommandResult).Stdout), &pairing)

				_ = channels
				_ = status
				_ = pairing
				_ = channelsErr
				_ = statusErr
				_ = pairingErr
				_ = unmarshalChannelsErr
				_ = unmarshalStatusErr
				_ = unmarshalPairingErr
			},
		},
		{
			name: "read-channel-logs",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "openclaw", &e2b.SandboxOpts{
					Envs: map[string]string{
						"OPENAI_API_KEY":     os.Getenv("OPENAI_API_KEY"),
						"TELEGRAM_BOT_TOKEN": os.Getenv("TELEGRAM_BOT_TOKEN"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "openclaw channels logs --channel telegram --lines 200", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 agents openclaw telegram doc snippets, got %d", got)
	}
}
