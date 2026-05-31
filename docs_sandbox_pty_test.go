package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxPtyDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/pty.mdx"); err != nil {
		t.Fatalf("sandbox pty doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/pty.mdx aligned with the exported Go SDK PTY
// surface. The closures are compile-only examples and are intentionally never
// executed.
func TestDocsSandboxPtyExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-pty",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
					Envs: map[string]string{
						"MY_VAR": "hello",
					},
					Cwd:  "/home/user",
					User: "root",
				})

				_ = terminal.Pid
				_ = ptyErr
			},
		},
		{
			name: "keep-open-timeout",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				timeoutMs := 0
				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      80,
					Rows:      24,
					TimeoutMs: &timeoutMs,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})

				_ = terminal.Pid
				_ = ptyErr
			},
		},
		{
			name: "send-input",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if ptyErr != nil {
					return
				}

				inputErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("echo 'hello from PTY'\n"), nil)

				_ = inputErr
			},
		},
		{
			name: "resize",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if ptyErr != nil {
					return
				}

				resizeErr := sandbox.Pty.Resize(ctx, terminal.Pid, e2b.PtySize{
					Cols: 120,
					Rows: 40,
				}, nil)

				_ = resizeErr
			},
		},
		{
			name: "disconnect-and-reconnect",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if ptyErr != nil {
					return
				}

				inputErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("echo hello\n"), nil)
				terminal.Disconnect()
				reconnected, connectErr := sandbox.Pty.Connect(ctx, terminal.Pid, &e2b.PtyConnectOpts{
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				sendErr := sandbox.Pty.SendInput(ctx, reconnected.Pid, []byte("echo world\n"), nil)
				result, waitErr := reconnected.Wait()

				_ = result
				_ = inputErr
				_ = connectErr
				_ = sendErr
				_ = waitErr
			},
		},
		{
			name: "kill-pty",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				terminal, ptyErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols: 80,
					Rows: 24,
				})
				if ptyErr != nil {
					return
				}

				killed, killErr := sandbox.Pty.Kill(ctx, terminal.Pid, nil)
				handleKilled, handleKillErr := terminal.Kill()

				_ = killed
				_ = handleKilled
				_ = killErr
				_ = handleKillErr
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 sandbox pty doc snippets, got %d", got)
	}
}
