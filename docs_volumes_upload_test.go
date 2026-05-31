package e2b_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesUploadDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/upload.mdx"); err != nil {
		t.Fatalf("volumes upload doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/upload.mdx aligned with the exported Go SDK
// volume upload surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesUploadExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "upload-single-file",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				file, openErr := os.Open("/local/path")
				if file != nil {
					defer file.Close()
				}

				info, writeErr := volume.WriteFile(ctx, "/path/in/volume", file, nil)

				_ = info
				_ = openErr
				_ = writeErr
			},
		},
		{
			name: "upload-directory",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				directoryPath := "/local/dir"
				entries, readDirErr := os.ReadDir(directoryPath)

				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}

					filePath := filepath.Join(directoryPath, entry.Name())
					file, openErr := os.Open(filePath)
					if openErr != nil {
						continue
					}

					_, _ = volume.WriteFile(ctx, "/upload/"+entry.Name(), file, nil)
					_ = file.Close()
				}

				_ = readDirErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 volumes upload doc snippets, got %d", got)
	}
}
