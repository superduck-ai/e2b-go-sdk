package doctest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxFilesystemReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox-filesystem.mdx"); err != nil {
		t.Fatalf("sandbox filesystem reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/sandbox-filesystem.mdx aligned
// with the exported Go SDK surface. The closures are compile-only examples and
// are intentionally never executed.
func TestDocsSandboxFilesystemReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "read",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				textValue, textErr := sandbox.Files.Read(ctx, "/tmp/app.txt", nil)
				text := textValue.(string)

				bytesValue, bytesErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})
				data := bytesValue.([]byte)

				blobValue, blobErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBlob,
				})
				blob := blobValue.(e2b.Blob)

				streamValue, streamErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatStream,
				})
				stream := streamValue.(io.ReadCloser)
				defer stream.Close()

				_, _ = io.ReadAll(stream)

				_ = text
				_ = data
				_ = blob
				_ = textErr
				_ = bytesErr
				_ = blobErr
				_ = streamErr
			},
		},
		{
			name: "write",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				blob := e2b.Blob([]byte("blob payload"))

				info1, err1 := sandbox.Files.Write(ctx, "/tmp/notes.txt", "hello from go", nil)
				info2, err2 := sandbox.Files.Write(ctx, "/tmp/data.bin", []byte("bytes payload"), nil)
				info3, err3 := sandbox.Files.Write(ctx, "/tmp/blob.txt", blob, nil)
				info4, err4 := sandbox.Files.Write(ctx, "/tmp/stream.txt", bytes.NewReader([]byte("stream payload")), &e2b.FilesystemWriteOpts{
					Gzip:           true,
					UseOctetStream: true,
				})

				infos, batchErr := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
					{Path: "/tmp/batch/one.txt", Data: "one"},
					{Path: "/tmp/batch/two.bin", Data: bytes.NewReader([]byte("two"))},
				}, nil)

				_ = info1
				_ = info2
				_ = info3
				_ = info4
				_ = infos
				_ = err1
				_ = err2
				_ = err3
				_ = err4
				_ = batchErr
			},
		},
		{
			name: "path-ops",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				created, createErr := sandbox.Files.MakeDir(ctx, "/tmp/project/data", nil)
				exists, existsErr := sandbox.Files.Exists(ctx, "/tmp/project/data", nil)

				depth := 2
				entries, listErr := sandbox.Files.List(ctx, "/tmp/project", &e2b.FilesystemListOpts{
					Depth: &depth,
				})

				info, infoErr := sandbox.Files.GetInfo(ctx, "/tmp/project", nil)
				renamed, renameErr := sandbox.Files.Rename(ctx, "/tmp/project/data", "/tmp/project/assets", nil)
				removeErr := sandbox.Files.Remove(ctx, "/tmp/project/assets", nil)

				_, notFoundErr := sandbox.Files.Read(ctx, "/tmp/missing.txt", nil)
				var fileErr *e2b.FileNotFoundError
				notFoundMatches := errors.As(notFoundErr, &fileErr)

				_ = created
				_ = exists
				_ = entries
				_ = info
				_ = renamed
				_ = notFoundMatches
				_ = createErr
				_ = existsErr
				_ = listErr
				_ = infoErr
				_ = renameErr
				_ = removeErr
			},
		},
		{
			name: "watch",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				watchTimeoutMs := 30_000
				handle, watchErr := sandbox.Files.WatchDir(ctx, "/tmp/project", func(event e2b.FilesystemEvent) {
					_ = event.Name
					_ = event.Type
				}, &e2b.WatchOpts{
					Recursive: true,
					TimeoutMs: &watchTimeoutMs,
					OnExit: func(err error) {
						_ = err
					},
				})
				if handle != nil {
					handle.Stop()
				}

				_ = e2b.FilesystemEventCreate
				_ = e2b.FilesystemEventWrite
				_ = e2b.FilesystemEventRename
				_ = e2b.FilesystemEventRemove
				_ = e2b.FilesystemEventChmod
				_ = watchErr
			},
		},
		{
			name: "request-options",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				timeoutMs := 15_000
				opts := &e2b.FilesystemRequestOpts{
					RequestTimeoutMs: &timeoutMs,
					User:             "root",
				}

				_, readErr := sandbox.Files.Read(ctx, "relative.txt", &e2b.FilesystemReadOpts{
					FilesystemRequestOpts: *opts,
					Gzip:                  true,
				})
				_, writeErr := sandbox.Files.Write(ctx, "relative.txt", "hello", &e2b.FilesystemWriteOpts{
					FilesystemRequestOpts: *opts,
				})
				_, listErr := sandbox.Files.List(ctx, ".", &e2b.FilesystemListOpts{
					FilesystemRequestOpts: *opts,
				})

				_ = readErr
				_ = writeErr
				_ = listErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 filesystem doc snippets, got %d", got)
	}
}
