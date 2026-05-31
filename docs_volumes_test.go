package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes.mdx"); err != nil {
		t.Fatalf("volumes overview doc is missing: %v", err)
	}
}

// This test keeps docs/volumes.mdx aligned with the exported Go SDK volume
// overview surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "volume-overview-example",
			fn: func() {
				ctx := context.Background()

				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				sandbox, createErr := e2b.Create(ctx, "", &e2b.SandboxOpts{
					VolumeMounts: map[string]any{
						"/mnt/my-data": volume,
					},
				})

				_ = volume
				_ = sandbox
				_ = createErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 volumes overview doc snippet, got %d", got)
	}
}
