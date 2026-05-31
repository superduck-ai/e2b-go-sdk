package doctest

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesReadWriteDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/read-write.mdx"); err != nil {
		t.Fatalf("volumes read-write doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/read-write.mdx aligned with the exported Go SDK
// volume content surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesReadWriteExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "read-file-formats",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				textValue, textErr := volume.ReadFile(ctx, "/path/to/file", nil)
				fileContent := textValue.(string)

				bytesValue, bytesErr := volume.ReadFile(ctx, "/path/to/file", &e2b.VolumeReadOpts{
					Format: e2b.ReadFileFormatBytes,
				})
				fileBytes := bytesValue.([]byte)

				blobValue, blobErr := volume.ReadFile(ctx, "/path/to/file", &e2b.VolumeReadOpts{
					Format: e2b.ReadFileFormatBlob,
				})
				fileBlob := blobValue.(e2b.Blob)

				streamValue, streamErr := volume.ReadFile(ctx, "/path/to/file", &e2b.VolumeReadOpts{
					Format: e2b.ReadFileFormatStream,
				})
				stream := streamValue.(io.ReadCloser)
				defer stream.Close()

				_ = fileContent
				_ = fileBytes
				_ = fileBlob
				_ = textErr
				_ = bytesErr
				_ = blobErr
				_ = streamErr
			},
		},
		{
			name: "write-file-inputs",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				textInfo, textErr := volume.WriteFile(ctx, "/path/to/file.txt", "file content", nil)
				bytesInfo, bytesErr := volume.WriteFile(ctx, "/path/to/file.bin", []byte("file content"), nil)
				blobInfo, blobErr := volume.WriteFile(ctx, "/path/to/blob.txt", e2b.Blob([]byte("blob content")), nil)
				streamInfo, streamErr := volume.WriteFile(ctx, "/path/to/stream.txt", bytes.NewReader([]byte("file content")), nil)

				_ = textInfo
				_ = bytesInfo
				_ = blobInfo
				_ = streamInfo
				_ = textErr
				_ = bytesErr
				_ = blobErr
				_ = streamErr
			},
		},
		{
			name: "make-dir",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				dirInfo, dirErr := volume.MakeDir(ctx, "/path/to/dir", nil)
				force := true
				nestedInfo, nestedErr := volume.MakeDir(ctx, "/path/to/nested/dir", &e2b.VolumeWriteOptions{
					Force: &force,
				})

				_ = dirInfo
				_ = nestedInfo
				_ = dirErr
				_ = nestedErr
			},
		},
		{
			name: "list-directory",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				depth := 2
				entries, listErr := volume.List(ctx, "/path/to/dir", &e2b.VolumeListOpts{
					Depth: &depth,
				})

				_ = entries
				_ = listErr
			},
		},
		{
			name: "remove-paths",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				fileErr := volume.Remove(ctx, "/path/to/file", nil)
				dirErr := volume.Remove(ctx, "/path/to/dir", nil)

				_ = fileErr
				_ = dirErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 volumes read-write doc snippets, got %d", got)
	}
}
