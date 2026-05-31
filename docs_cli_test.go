package e2b_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsCliDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli.mdx"); err != nil {
		t.Fatalf("cli overview doc is missing: %v", err)
	}
}

// This overview page is intentionally prose-only because the CLI binary is a
// separate tool and this repository documents the Go SDK equivalents.
func TestDocsCliExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 cli overview doc snippets, got %d", got)
	}
}

func TestDocsCliAuthDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/auth.mdx"); err != nil {
		t.Fatalf("cli auth doc is missing: %v", err)
	}
}

// This test keeps docs/cli/auth.mdx aligned with the exported Go SDK auth and
// connection surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliAuthExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "env-based-auth",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
			},
		},
		{
			name: "explicit-connection-opts",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "base", &e2b.SandboxOpts{
					ConnectionOpts: e2b.ConnectionOpts{
						ApiKey:      os.Getenv("E2B_API_KEY"),
						AccessToken: os.Getenv("E2B_ACCESS_TOKEN"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 cli auth doc snippets, got %d", got)
	}
}

func TestDocsCliConnectToSandboxDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/connect-to-sandbox.mdx"); err != nil {
		t.Fatalf("cli connect-to-sandbox doc is missing: %v", err)
	}
}

// This test keeps docs/cli/connect-to-sandbox.mdx aligned with the exported Go
// SDK connect and PTY surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliConnectToSandboxExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "connect-and-start-pty",
			fn: func() {
				ctx := context.Background()
				ptyTimeoutMs := 0

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				terminal, createErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      120,
					Rows:      36,
					TimeoutMs: &ptyTimeoutMs,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if createErr != nil {
					return
				}

				sendErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("pwd\n"), nil)

				_ = sendErr
			},
		},
		{
			name: "reconnect-existing-pty",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				terminal, connectErr := sandbox.Pty.Connect(ctx, 1234, &e2b.PtyConnectOpts{
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if terminal != nil {
					terminal.Disconnect()
				}

				_ = connectErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 cli connect-to-sandbox doc snippets, got %d", got)
	}
}

func TestDocsCliCreateSandboxDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/create-sandbox.mdx"); err != nil {
		t.Fatalf("cli create-sandbox doc is missing: %v", err)
	}
}

// This test keeps docs/cli/create-sandbox.mdx aligned with the exported Go SDK
// create and PTY surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliCreateSandboxExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-and-open-terminal",
			fn: func() {
				ctx := context.Background()
				ptyTimeoutMs := 0

				sandbox, err := e2b.Create(ctx, "base", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				terminal, createErr := sandbox.Pty.Create(ctx, &e2b.PtyCreateOpts{
					Cols:      120,
					Rows:      36,
					TimeoutMs: &ptyTimeoutMs,
					OnData: func(data e2b.PtyOutput) {
						_ = string(data)
					},
				})
				if createErr != nil {
					return
				}

				sendErr := sandbox.Pty.SendInput(ctx, terminal.Pid, []byte("whoami\n"), nil)
				killed, killErr := terminal.Kill()

				_ = sendErr
				_ = killed
				_ = killErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 cli create-sandbox doc snippet, got %d", got)
	}
}

func TestDocsCliExecCommandDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/exec-command.mdx"); err != nil {
		t.Fatalf("cli exec-command doc is missing: %v", err)
	}
}

// This test keeps docs/cli/exec-command.mdx aligned with the exported Go SDK
// commands surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliExecCommandExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "foreground-command",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "echo hello", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "stdin-streaming",
			fn: func() {
				ctx := context.Background()
				stdin := true

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "cat", &e2b.CommandStartOpts{
					Background: true,
					Stdin:      &stdin,
				})
				handle := execution.(*e2b.CommandHandle)
				sendErr := sandbox.Commands.SendStdin(ctx, handle.Pid, []byte("foo\n"), nil)
				closeErr := sandbox.Commands.CloseStdin(ctx, handle.Pid, nil)
				result, waitErr := handle.Wait()

				_ = result
				_ = runErr
				_ = sendErr
				_ = closeErr
				_ = waitErr
			},
		},
		{
			name: "background-cwd-user-env",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				execution, runErr := sandbox.Commands.Run(ctx, "node app.js", &e2b.CommandStartOpts{
					Background: true,
					Cwd:        "/home/user",
					User:       "root",
					Envs: map[string]string{
						"NODE_ENV": "production",
						"DEBUG":    "true",
					},
				})
				handle := execution.(*e2b.CommandHandle)

				_ = handle.Pid
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 cli exec-command doc snippets, got %d", got)
	}
}

func TestDocsCliListSandboxesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/list-sandboxes.mdx"); err != nil {
		t.Fatalf("cli list-sandboxes doc is missing: %v", err)
	}
}

// This test keeps docs/cli/list-sandboxes.mdx aligned with the exported Go SDK
// list paginator surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliListSandboxesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "basic-listing",
			fn: func() {
				paginator := e2b.List(nil)
				page, listErr := paginator.NextItems()

				_ = page
				_ = listErr
			},
		},
		{
			name: "filters-and-limit",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{
							"key1": "value1",
							"key2": "value2",
						},
						State: []e2b.SandboxState{
							e2b.SandboxState("running"),
							e2b.SandboxState("paused"),
						},
					},
					Limit: 10,
				})
				page, listErr := paginator.NextItems()

				_ = page
				_ = listErr
			},
		},
		{
			name: "json-output",
			fn: func() {
				paginator := e2b.List(nil)
				page, listErr := paginator.NextItems()
				payload, jsonErr := json.MarshalIndent(page, "", "  ")

				_ = payload
				_ = listErr
				_ = jsonErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 cli list-sandboxes doc snippets, got %d", got)
	}
}

func TestDocsCliShutdownSandboxesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cli/shutdown-sandboxes.mdx"); err != nil {
		t.Fatalf("cli shutdown-sandboxes doc is missing: %v", err)
	}
}

// This test keeps docs/cli/shutdown-sandboxes.mdx aligned with the exported Go
// SDK list and kill surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsCliShutdownSandboxesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "kill-known-ids",
			fn: func() {
				ctx := context.Background()
				ids := []string{"sbx_1", "sbx_2", "sbx_3"}

				for _, sandboxID := range ids {
					killed, killErr := e2b.Kill(ctx, sandboxID, nil)
					_ = killed
					_ = killErr
				}
			},
		},
		{
			name: "kill-all-matching",
			fn: func() {
				ctx := context.Background()

				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{
							"key": "value",
						},
						State: []e2b.SandboxState{
							e2b.SandboxState("running"),
							e2b.SandboxState("paused"),
						},
					},
				})

				for paginator.HasNext {
					page, listErr := paginator.NextItems()
					_ = listErr

					for _, sandbox := range page {
						killed, killErr := e2b.Kill(ctx, sandbox.SandboxID, nil)
						_ = killed
						_ = killErr
					}
				}
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 cli shutdown-sandboxes doc snippets, got %d", got)
	}
}
