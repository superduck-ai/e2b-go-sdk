package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemWatchDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/watch.mdx"); err != nil {
		t.Fatalf("filesystem watch doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem/watch.mdx aligned with the exported Go SDK
// watch surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsFilesystemWatchExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "watch-directory",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				handle, watchErr := sandbox.Files.WatchDir(ctx, "/home/user", func(event e2b.FilesystemEvent) {
					_ = event.Name
					_ = event.Type
				}, nil)
				if handle != nil {
					defer handle.Stop()
				}

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/my-file", "hello", nil)
				_ = watchErr
				_ = writeErr
			},
		},
		{
			name: "watch-directory-recursive",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

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
				if handle != nil {
					defer handle.Stop()
				}

				_, writeErr := sandbox.Files.Write(ctx, "/home/user/my-folder/my-file", "hello", nil)

				_ = e2b.FilesystemEventCreate
				_ = e2b.FilesystemEventWrite
				_ = e2b.FilesystemEventRename
				_ = e2b.FilesystemEventRemove
				_ = e2b.FilesystemEventChmod
				_ = watchErr
				_ = writeErr
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem watch doc snippets, got %d", got)
	}
}
