package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsAPIKeyDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/api-key.mdx"); err != nil {
		t.Fatalf("api-key doc is missing: %v", err)
	}
}

// This test keeps docs/api-key.mdx aligned with the exported Go SDK
// authentication surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsAPIKeyExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "pass-api-key-in-opts",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "", &e2b.SandboxOpts{
					ConnectionOpts: e2b.ConnectionOpts{
						ApiKey: "YOUR_API_KEY",
					},
				})

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 api-key doc snippet, got %d", got)
	}
}
