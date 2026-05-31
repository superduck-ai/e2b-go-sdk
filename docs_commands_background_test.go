package e2b_test

import (
	"context"
	"errors"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCommandsBackgroundDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/commands/background.mdx"); err != nil {
		t.Fatalf("commands background doc is missing: %v", err)
	}
}

// This test keeps docs/commands/background.mdx aligned with the exported Go
// SDK background-command surface. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsCommandsBackgroundExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "start-background-command",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello; sleep 10; echo world", &e2b.CommandStartOpts{
					Background: true,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
				})

				command := execution.(*e2b.CommandHandle)
				state := command.State()

				_ = state.Stdout
				_ = state.Stderr
				_ = state.ExitCode
				_ = state.Error
				_, _ = command.Kill()
				_ = runErr
			},
		},
		{
			name: "wait-for-background-command",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "sleep 1 && echo done", &e2b.CommandStartOpts{
					Background: true,
				})
				handle := execution.(*e2b.CommandHandle)

				result, waitErr := handle.Wait()
				var exitErr *e2b.CommandExitError
				_ = errors.As(waitErr, &exitErr)

				_ = result
				_ = runErr
			},
		},
		{
			name: "reconnect-and-kill",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				processes, listErr := sandbox.Commands.List(ctx, nil)
				if len(processes) == 0 {
					_ = listErr
					return
				}

				handle, connectErr := sandbox.Commands.Connect(ctx, processes[0].Pid, nil)
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
				_ = connectErr
				_ = killErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 commands background doc snippets, got %d", got)
	}
}
