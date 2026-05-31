package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxMetadataDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/metadata.mdx"); err != nil {
		t.Fatalf("sandbox metadata doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/metadata.mdx aligned with the exported Go SDK
// metadata surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxMetadataExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "set-and-read-metadata",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					Metadata: map[string]string{
						"userID": "123",
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{
							"userID": "123",
						},
					},
				})

				items, listErr := paginator.NextItems()
				var metadata map[string]string
				if len(items) > 0 {
					metadata = items[0].Metadata
				}

				_ = sandbox
				_ = metadata
				_ = listErr
			},
		},
		{
			name: "filter-by-metadata",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{
							"userID": "123",
							"env":    "dev",
						},
					},
				})

				sandboxes, err := paginator.NextItems()

				_ = sandboxes
				_ = err
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 sandbox metadata doc snippets, got %d", got)
	}
}
