package doctest

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemReadWriteDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/read-write.mdx"); err != nil {
		t.Fatalf("filesystem read-write doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem/read-write.mdx aligned with the exported Go
// SDK read/write filesystem surface. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsFilesystemReadWriteExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "read-files",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				textValue, textErr := sandbox.Files.Read(ctx, "/path/to/file", nil)
				fileContent := textValue.(string)

				bytesValue, bytesErr := sandbox.Files.Read(ctx, "/path/to/file", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})
				fileBytes := bytesValue.([]byte)

				streamValue, streamErr := sandbox.Files.Read(ctx, "/path/to/file", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatStream,
				})
				stream := streamValue.(io.ReadCloser)
				defer stream.Close()

				_ = fileContent
				_ = fileBytes
				_ = textErr
				_ = bytesErr
				_ = streamErr
			},
		},
		{
			name: "write-single-file",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				_, err1 := sandbox.Files.Write(ctx, "/path/to/file.txt", "file content", nil)
				_, err2 := sandbox.Files.Write(ctx, "/path/to/file.bin", []byte("file content"), nil)
				_, err3 := sandbox.Files.Write(ctx, "/path/to/stream.txt", bytes.NewReader([]byte("file content")), nil)

				_ = err1
				_ = err2
				_ = err3
			},
		},
		{
			name: "write-multiple-files",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				infos, batchErr := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
					{Path: "/path/to/a", Data: "file content"},
					{Path: "/another/path/to/b", Data: "file content"},
				}, nil)

				_ = infos
				_ = batchErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 filesystem read-write doc snippets, got %d", got)
	}
}
