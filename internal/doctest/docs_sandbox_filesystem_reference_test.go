package doctest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxFilesystemReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox-filesystem.mdx"); err != nil {
		t.Fatalf("sandbox filesystem reference doc is missing: %v", err)
	}
}

func TestDocsSandboxFilesystemReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "read",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				textValue, textErr := sandbox.Files.Read(ctx, "/tmp/app.txt", nil)
				if !assert.NoError(t, textErr, "read text") {
					return
				}
				text := textValue.(string)

				bytesValue, bytesErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBytes,
				})
				if !assert.NoError(t, bytesErr, "read bytes") {
					return
				}
				data := bytesValue.([]byte)

				blobValue, blobErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatBlob,
				})
				if !assert.NoError(t, blobErr, "read blob") {
					return
				}
				blob := blobValue.(e2b.Blob)

				streamValue, streamErr := sandbox.Files.Read(ctx, "/tmp/app.txt", &e2b.FilesystemReadOpts{
					Format: e2b.ReadFormatStream,
				})
				if !assert.NoError(t, streamErr, "read stream") {
					return
				}
				stream := streamValue.(io.ReadCloser)
				defer stream.Close()

				_, _ = io.ReadAll(stream)

				_ = text
				_ = data
				_ = blob
			},
		},
		{
			name: "write",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				blob := e2b.Blob([]byte("blob payload"))

				info1, err1 := sandbox.Files.Write(ctx, "/tmp/notes.txt", "hello from go", nil)
				assert.NoError(t, err1, "write string")
				info2, err2 := sandbox.Files.Write(ctx, "/tmp/data.bin", []byte("bytes payload"), nil)
				assert.NoError(t, err2, "write bytes")
				info3, err3 := sandbox.Files.Write(ctx, "/tmp/blob.txt", blob, nil)
				assert.NoError(t, err3, "write blob")
				info4, err4 := sandbox.Files.Write(ctx, "/tmp/stream.txt", bytes.NewReader([]byte("stream payload")), &e2b.FilesystemWriteOpts{
					Gzip:           true,
					UseOctetStream: true,
				})
				assert.NoError(t, err4, "write stream")

				infos, batchErr := sandbox.Files.WriteFiles(ctx, []e2b.WriteEntry{
					{Path: "/tmp/batch/one.txt", Data: "one"},
					{Path: "/tmp/batch/two.bin", Data: bytes.NewReader([]byte("two"))},
				}, nil)
				assert.NoError(t, batchErr, "write batch")

				_ = info1
				_ = info2
				_ = info3
				_ = info4
				_ = infos
			},
		},
		{
			name: "path-ops",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				created, createErr := sandbox.Files.MakeDir(ctx, "/tmp/project/data", nil)
				assert.NoError(t, createErr, "mkdir")
				exists, existsErr := sandbox.Files.Exists(ctx, "/tmp/project/data", nil)
				assert.NoError(t, existsErr, "exists")

				depth := 2
				entries, listErr := sandbox.Files.List(ctx, "/tmp/project", &e2b.FilesystemListOpts{
					Depth: &depth,
				})
				assert.NoError(t, listErr, "list")

				info, infoErr := sandbox.Files.GetInfo(ctx, "/tmp/project", nil)
				assert.NoError(t, infoErr, "info")
				renamed, renameErr := sandbox.Files.Rename(ctx, "/tmp/project/data", "/tmp/project/assets", nil)
				assert.NoError(t, renameErr, "rename")
				assert.NoError(t, sandbox.Files.Remove(ctx, "/tmp/project/assets", nil), "remove")

				_, notFoundErr := sandbox.Files.Read(ctx, "/tmp/missing.txt", nil)
				var fileErr *e2b.FileNotFoundError
				notFoundMatches := errors.As(notFoundErr, &fileErr)

				_ = created
				_ = exists
				_ = entries
				_ = info
				_ = renamed
				_ = notFoundMatches
			},
		},
		{
			name: "watch",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
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
				assert.NoError(t, watchErr, "watch")
				if handle != nil {
					handle.Stop()
				}

				_ = e2b.FilesystemEventCreate
				_ = e2b.FilesystemEventWrite
				_ = e2b.FilesystemEventRename
				_ = e2b.FilesystemEventRemove
				_ = e2b.FilesystemEventChmod
			},
		},
		{
			name: "request-options",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
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
				assert.NoError(t, readErr, "read with opts")
				_, writeErr := sandbox.Files.Write(ctx, "relative.txt", "hello", &e2b.FilesystemWriteOpts{
					FilesystemRequestOpts: *opts,
				})
				assert.NoError(t, writeErr, "write with opts")
				_, listErr := sandbox.Files.List(ctx, ".", &e2b.FilesystemListOpts{
					FilesystemRequestOpts: *opts,
				})
				assert.NoError(t, listErr, "list with opts")
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 filesystem doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
