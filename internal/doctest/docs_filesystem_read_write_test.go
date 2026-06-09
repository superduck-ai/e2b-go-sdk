package doctest

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemReadWriteDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/read-write.mdx"); err != nil {
		t.Fatalf("filesystem read-write doc is missing: %v", err)
	}
}

func TestDocsFilesystemReadWriteExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "read-files",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				path := "/home/user/read-target.txt"
				_, prepErr := sandbox.Files.Write(ctx, path, "file content", nil)
				if !assert.NoError(t, prepErr, "failed to prepare file") {
					return
				}

				textValue, textErr := sandbox.Files.Read(ctx, path, nil)
				if !assert.NoError(t, textErr, "failed to read text") {
					return
				}
				fileContent := textValue.(string)

				bytesValue, bytesErr := sandbox.Files.Read(ctx, path, &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})
				if !assert.NoError(t, bytesErr, "failed to read bytes") {
					return
				}
				fileBytes := bytesValue.([]byte)

				streamValue, streamErr := sandbox.Files.Read(ctx, path, &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatStream,
				})
				if !assert.NoError(t, streamErr, "failed to read stream") {
					return
				}
				stream := streamValue.(io.ReadCloser)
				defer stream.Close()

				_ = fileContent
				_ = fileBytes
			},
		},
		{
			name: "write-single-file",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, err1 := sandbox.Files.Write(ctx, "/home/user/file.txt", "file content", nil)
				assert.NoError(t, err1, "failed to write string")

				_, err2 := sandbox.Files.Write(ctx, "/home/user/file.bin", []byte("file content"), nil)
				assert.NoError(t, err2, "failed to write bytes")

				_, err3 := sandbox.Files.Write(ctx, "/home/user/stream.txt", bytes.NewReader([]byte("file content")), nil)
				assert.NoError(t, err3, "failed to write stream")
			},
		},
		{
			name: "write-multiple-files",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				infos, batchErr := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
					{Path: "/home/user/a", Data: "file content"},
					{Path: "/home/user/sub/b", Data: "file content"},
				}, nil)
				assert.NoError(t, batchErr, "failed to write files in batch")

				_ = infos
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 filesystem read-write doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
