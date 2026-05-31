package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxPersistenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/persistence.mdx"); err != nil {
		t.Fatalf("sandbox persistence doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/persistence.mdx aligned with the exported Go
// SDK pause, connect, and lifecycle surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxPersistenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "pause-resume-kill",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				paused, pauseErr := sandbox.Pause(ctx, nil)
				sameSandbox, connectErr := sandbox.Connect(ctx, nil)
				killErr := sandbox.Kill(ctx, nil)

				_ = paused
				_ = sameSandbox
				_ = pauseErr
				_ = connectErr
				_ = killErr
			},
		},
		{
			name: "list-paused",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						State: []e2b.SandboxState{e2b.SandboxState("paused")},
					},
				})

				sandboxes, err := paginator.NextItems()

				_ = sandboxes
				_ = err
			},
		},
		{
			name: "remove-paused-sandbox",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				killErr := sandbox.Kill(ctx, nil)
				deleted, deleteErr := e2b.Kill(ctx, "sbx_123", nil)

				_ = deleted
				_ = killErr
				_ = deleteErr
			},
		},
		{
			name: "connect-timeout",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 60_000

				sandbox, err := e2b.Connect(ctx, "sbx_123", &e2b.SandboxConnectOpts{
					TimeoutMs: &timeoutMs,
				})

				_ = sandbox
				_ = err
			},
		},
		{
			name: "auto-pause-config",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 10 * 60 * 1000
				autoResume := false

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Lifecycle: &e2b.SandboxLifecycle{
						OnTimeout:  "pause",
						AutoResume: &autoResume,
					},
				})
				if sandbox != nil {
					defer sandbox.Kill(context.Background(), nil)
				}

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 sandbox persistence doc snippets, got %d", got)
	}
}
