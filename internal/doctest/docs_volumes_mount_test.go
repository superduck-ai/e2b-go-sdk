package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesMountDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/mount.mdx"); err != nil {
		t.Fatalf("volumes mount doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/mount.mdx aligned with the exported Go SDK
// volume mounting surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesMountExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "mount-volume-object",
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
				if createErr != nil {
					return
				}

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/mnt/my-data/hello.txt", "Hello, world!", nil)

				_ = writeInfo
				_ = writeErr
			},
		},
		{
			name: "mount-by-name",
			fn: func() {
				ctx := context.Background()

				sandbox, createErr := e2b.Create(ctx, "", &e2b.SandboxOpts{
					VolumeMounts: map[string]any{
						"/mnt/my-data": "my-volume",
					},
				})
				if createErr != nil {
					return
				}

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/mnt/my-data/hello.txt", "Hello, world!", nil)

				_ = writeInfo
				_ = writeErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 volumes mount doc snippets, got %d", got)
	}
}
