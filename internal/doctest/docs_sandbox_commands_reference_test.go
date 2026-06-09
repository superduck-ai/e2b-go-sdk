package doctest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxCommandsReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox-commands.mdx"); err != nil {
		t.Fatalf("sandbox commands reference doc is missing: %v", err)
	}
}

func TestDocsSandboxCommandsReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "foreground-run",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello", nil)
				if !assert.NoError(t, runErr, "failed to run") {
					return
				}
				result := execution.(*e2b.CommandResult)

				_ = result.ExitCode
				_ = result.Stdout
				_ = result.Stderr
			},
		},
		{
			name: "background-run-and-state",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
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
				if !assert.NoError(t, runErr, "failed to start bg command") {
					return
				}
				handle := execution.(*e2b.CommandHandle)
				assert.NoError(t, sandbox.Commands.SendStdin(ctx, handle.Pid, []byte("hello\n"), nil), "send stdin")
				assert.NoError(t, sandbox.Commands.CloseStdin(ctx, handle.Pid, nil), "close stdin")

				state := handle.State()
				_ = state.Stdout
				_ = state.Stderr
				_ = state.ExitCode
				_ = state.Error

				killed, killErr := handle.Kill()
				assert.NoError(t, killErr, "kill")
				_ = killed

				result, waitErr := handle.Wait()
				var exitErr *e2b.CommandExitError
				_ = errors.As(waitErr, &exitErr)
				_ = result
			},
		},
		{
			name: "list-and-connect",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				processes, listErr := sandbox.Commands.List(ctx, nil)
				if !assert.NoError(t, listErr, "failed to list") {
					return
				}
				if len(processes) == 0 {
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
				assert.NoError(t, connectErr, "failed to connect to command")
				if handle != nil {
					handle.Disconnect()
				}

				killed, killErr := sandbox.Commands.Kill(ctx, processes[0].Pid, nil)
				assert.NoError(t, killErr, "kill command")
				_ = killed
			},
		},
		{
			name: "pty-size-type",
			fn: func(t *testing.T) {
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
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
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
				if !assert.NoError(t, createErr, "failed to create pty") {
					return
				}
				if handle != nil {
					assert.NoError(t, sandbox.Pty.Resize(ctx, handle.Pid, e2b.PtySize{Cols: 100, Rows: 30}, nil), "resize")
					assert.NoError(t, sandbox.Pty.SendInput(ctx, handle.Pid, []byte("echo $FOO\n"), nil), "send input")
					killed, killErr := sandbox.Pty.Kill(ctx, handle.Pid, nil)
					assert.NoError(t, killErr, "kill pty")
					_ = killed

					reconnected, connectErr := sandbox.Pty.Connect(ctx, handle.Pid, &e2b.PtyConnectOpts{
						OnData: func(data e2b.PtyOutput) {
							_ = data
						},
					})
					assert.NoError(t, connectErr, "reconnect pty")
					if reconnected != nil {
						reconnected.Disconnect()
					}
				}
			},
		},
		{
			name: "option-shapes",
			fn: func(t *testing.T) {
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

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
