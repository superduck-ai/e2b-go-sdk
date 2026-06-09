package doctest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsBackgroundDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands/background.mdx"); err != nil {
		t.Fatalf("commands background doc is missing: %v", err)
	}
}

func TestDocsCommandsBackgroundExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "start-background-command",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello; sleep 10; echo world", &e2b.CommandStartOpts{
					Background: true,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
				})
				if !assert.NoError(t, runErr, "failed to start background command") {
					return
				}

				command := execution.(*e2b.CommandHandle)
				state := command.State()

				_ = state.Stdout
				_ = state.Stderr
				_ = state.ExitCode
				_ = state.Error
				_, _ = command.Kill()
			},
		},
		{
			name: "wait-for-background-command",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect to sandbox") {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "sleep 1 && echo done", &e2b.CommandStartOpts{
					Background: true,
				})
				if !assert.NoError(t, runErr, "failed to start background command") {
					return
				}
				handle := execution.(*e2b.CommandHandle)

				result, waitErr := handle.Wait()
				var exitErr *e2b.CommandExitError
				_ = errors.As(waitErr, &exitErr)

				_ = result
			},
		},
		{
			name: "reconnect-and-kill",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect to sandbox") {
					return
				}

				processes, listErr := sandbox.Commands.List(ctx, nil)
				if !assert.NoError(t, listErr, "failed to list commands") {
					return
				}
				if len(processes) == 0 {
					return
				}

				handle, connectErr := sandbox.Commands.Connect(ctx, processes[0].Pid, nil)
				assert.NoError(t, connectErr, "failed to connect to command")
				if handle != nil {
					handle.Disconnect()
				}

				_ = processes[0].Pid
				_ = processes[0].Tag
				_ = processes[0].Cmd
				_ = processes[0].Args
				_ = processes[0].Envs
				_ = processes[0].Cwd

				_, killErr := sandbox.Commands.Kill(ctx, processes[0].Pid, nil)
				assert.NoError(t, killErr, "failed to kill command")
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 commands background doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
