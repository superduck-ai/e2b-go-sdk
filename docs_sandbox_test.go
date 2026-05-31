package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox.mdx"); err != nil {
		t.Fatalf("sandbox lifecycle doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox.mdx aligned with the exported Go SDK sandbox
// lifecycle surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-with-timeout",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 60_000

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
			},
		},
		{
			name: "change-timeout",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				setTimeoutErr := sandbox.SetTimeout(ctx, 30_000, nil)

				_ = setTimeoutErr
			},
		},
		{
			name: "get-info",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				info, infoErr := sandbox.GetInfo(ctx, nil)
				if infoErr != nil {
					return
				}

				_ = info.SandboxID
				_ = info.State
				_ = info.EndAt
			},
		},
		{
			name: "kill",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				killErr := sandbox.Kill(ctx, nil)

				_ = killErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 sandbox lifecycle doc snippets, got %d", got)
	}
}
