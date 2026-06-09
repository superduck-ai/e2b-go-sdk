package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox.mdx"); err != nil {
		t.Fatalf("sandbox lifecycle doc is missing: %v", err)
	}
}

func TestDocsSandboxExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "create-with-timeout",
			fn: func(t *testing.T) {
				ctx := context.Background()
				timeoutMs := 60_000

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
				})
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
			},
		},
		{
			name: "change-timeout",
			fn: func(t *testing.T) {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				setTimeoutErr := sandbox.SetTimeout(ctx, 30_000, nil)
				assert.NoError(t, setTimeoutErr, "failed to set timeout")
			},
		},
		{
			name: "get-info",
			fn: func(t *testing.T) {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				info, infoErr := sandbox.GetInfo(ctx, nil)
				if !assert.NoError(t, infoErr, "failed to get info") {
					return
				}

				_ = info.SandboxID
				_ = info.State
				_ = info.EndAt
			},
		},
		{
			name: "kill",
			fn: func(t *testing.T) {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}

				killErr := sandbox.Kill(ctx, nil)
				assert.NoError(t, killErr, "failed to kill sandbox")
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 sandbox lifecycle doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
