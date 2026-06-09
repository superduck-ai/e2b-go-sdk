package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemWatchDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/watch.mdx"); err != nil {
		t.Fatalf("filesystem watch doc is missing: %v", err)
	}
}

func TestDocsFilesystemWatchExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "watch-directory",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				handle, watchErr := sandbox.Files.WatchDir(ctx, "/home/user", func(event e2b.FilesystemEvent) {
					_ = event.Name
					_ = event.Type
				}, nil)
				if !assert.NoError(t, watchErr, "failed to watch dir") {
					return
				}
				if handle != nil {
					defer handle.Stop()
				}

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/my-file", "hello", nil)
				assert.NoError(t, writeErr, "failed to write file")
			},
		},
		{
			name: "watch-directory-recursive",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				timeoutMs := 30_000
				handle, watchErr := sandbox.Files.WatchDir(ctx, "/home/user", func(event e2b.FilesystemEvent) {
					_ = event.Name
					_ = event.Type
				}, &e2b.WatchOpts{
					Recursive: true,
					TimeoutMs: &timeoutMs,
					OnExit: func(err error) {
						_ = err
					},
				})
				if !assert.NoError(t, watchErr, "failed to watch dir (recursive)") {
					return
				}
				if handle != nil {
					defer handle.Stop()
				}

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/my-folder/my-file", "hello", nil)
				assert.NoError(t, writeErr, "failed to write file in subfolder")

				_ = e2b.FilesystemEventCreate
				_ = e2b.FilesystemEventWrite
				_ = e2b.FilesystemEventRename
				_ = e2b.FilesystemEventRemove
				_ = e2b.FilesystemEventChmod
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem watch doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
