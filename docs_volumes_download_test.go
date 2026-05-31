package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesDownloadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/download.mdx"); err != nil {
		t.Fatalf("volumes download doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/download.mdx aligned with the exported Go SDK
// volume download surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesDownloadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "download-bytes",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				value, readErr := volume.ReadFile(ctx, "/path/in/volume", &e2b.VolumeReadOpts{
					Format: e2b.ReadFileFormatBytes,
				})
				content := value.([]byte)
				writeErr := os.WriteFile("/local/path", content, 0o644)

				_ = readErr
				_ = writeErr
			},
		},
		{
			name: "download-text",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				value, readErr := volume.ReadFile(ctx, "/path/in/volume", nil)
				content := value.(string)
				writeErr := os.WriteFile("/local/path.txt", []byte(content), 0o644)

				_ = readErr
				_ = writeErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 volumes download doc snippets, got %d", got)
	}
}
