package doctest

import (
	"context"
	"errors"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxCommandsReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox-commands.mdx"); err != nil {
		t.Fatalf("sandbox commands reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/sandbox-commands.mdx aligned with
// the exported Go SDK surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxCommandsReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "foreground-run",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.ExitCode
				_ = result.Stdout
				_ = result.Stderr
				_ = runErr
			},
		},
		{
			name: "background-run-and-state",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				stdin := true
				execution, runErr := sandbox.Commands.Run(ctx, "sleep 30", &e2b.CommandStartOpts{
					Background: true,
					Stdin:      &stdin,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})
				handle := execution.(*e2b.CommandHandle)
				sendErr := sandbox.Commands.SendStdin(ctx, handle.Pid, []byte("hello\n"), nil)
				closeErr := sandbox.Commands.CloseStdin(ctx, handle.Pid, nil)

				state := handle.State()
				_ = state.Stdout
				_ = state.Stderr
				_ = state.ExitCode
				_ = state.Error

				killed, killErr := handle.Kill()
				_ = killed

				result, waitErr := handle.Wait()
				var exitErr *e2b.CommandExitError
				_ = errors.As(waitErr, &exitErr)

				_ = result
				_ = runErr
				_ = sendErr
				_ = closeErr
				_ = killErr
			},
		},
		{
			name: "list-and-connect",
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

				_ = processes[0].Pid
				_ = processes[0].Tag
				_ = processes[0].Cmd
				_ = processes[0].Args
				_ = processes[0].Envs
				_ = processes[0].Cwd

				handle, connectErr := sandbox.Commands.Connect(ctx, processes[0].Pid, &e2b.CommandConnectOpts{
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
				})
				if handle != nil {
					handle.Disconnect()
				}

				killed, killErr := sandbox.Commands.Kill(ctx, processes[0].Pid, nil)
				_ = killed
				_ = listErr
				_ = connectErr
				_ = killErr
			},
		},
		{
			name: "pty-size-type",
			fn: func() {
				size := e2b.PtySize{
					Cols: 80,
					Rows: 24,
				}

				_ = size.Cols
				_ = size.Rows
			},
		},
		{
			name: "pty",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				handle, createErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
					Cwd:  "/tmp",
					Envs: map[string]string{"FOO": "bar"},
					OnData: func(data e2b.PtyOutput) {
						_ = data
					},
				})
				if handle != nil {
					resizeErr := sandbox.Pty.Resize(ctx, handle.Pid, e2b.PtySize{Cols: 100, Rows: 30}, nil)
					sendErr := sandbox.Pty.SendInput(ctx, handle.Pid, []byte("echo $FOO\n"), nil)
					killed, killErr := sandbox.Pty.Kill(ctx, handle.Pid, nil)
					reconnected, connectErr := sandbox.Pty.Connect(ctx, handle.Pid, &e2b.PtyConnectOpts{
						OnData: func(data e2b.PtyOutput) {
							_ = data
						},
					})
					if reconnected != nil {
						reconnected.Disconnect()
					}

					_ = resizeErr
					_ = sendErr
					_ = killed
					_ = killErr
					_ = connectErr
				}

				_ = createErr
			},
		},
		{
			name: "option-shapes",
			fn: func() {
				timeoutMs := 30_000
				requestTimeoutMs := 15_000
				signal := context.Background()

				commandRequestOpts := e2b.CommandRequestOpts{
					RequestTimeoutMs: &requestTimeoutMs,
					Signal:           signal,
				}

				commandStartOpts := e2b.CommandStartOpts{
					CommandRequestOpts: commandRequestOpts,
					Background:         true,
					Cwd:                "/tmp",
					User:               "root",
					Envs:               map[string]string{"FOO": "bar"},
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
					TimeoutMs: &timeoutMs,
				}

				commandConnectOpts := e2b.CommandConnectOpts{
					CommandRequestOpts: commandRequestOpts,
					OnStdout: func(data e2b.Stdout) {
						_ = data
					},
					OnStderr: func(data e2b.Stderr) {
						_ = data
					},
					TimeoutMs: &timeoutMs,
				}

				ptyCreateOpts := e2b.PtyCreateOpts{
					Cols:             80,
					Rows:             24,
					TimeoutMs:        &timeoutMs,
					Signal:           signal,
					RequestTimeoutMs: &requestTimeoutMs,
				}

				ptyConnectOpts := e2b.PtyConnectOpts{
					TimeoutMs:        &timeoutMs,
					Signal:           signal,
					RequestTimeoutMs: &requestTimeoutMs,
				}

				_ = commandRequestOpts
				_ = commandStartOpts
				_ = commandConnectOpts
				_ = ptyCreateOpts
				_ = ptyConnectOpts
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 commands doc snippets, got %d", got)
	}
}
