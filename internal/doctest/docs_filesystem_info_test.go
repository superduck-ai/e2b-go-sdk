package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemInfoDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/info.mdx"); err != nil {
		t.Fatalf("filesystem info doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem/info.mdx aligned with the exported Go SDK
// file-info surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsFilesystemInfoExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "file-info",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				_, _ = sandbox.Files.Write(ctx, "test_file.txt", "Hello, world!", nil)
				info, _ := sandbox.Files.GetInfo(ctx, "test_file.txt", nil)

				if info != nil {
					_ = info.Name
					_ = info.Type
					_ = info.Path
					_ = info.Size
					_ = info.Mode
					_ = info.Permissions
					_ = info.Owner
					_ = info.Group
					_ = info.ModifiedTime
					_ = info.SymlinkTarget
				}
			},
		},
		{
			name: "directory-info",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				_, _ = sandbox.Files.MakeDir(ctx, "test_dir", nil)
				info, _ := sandbox.Files.GetInfo(ctx, "test_dir", nil)

				if info != nil {
					_ = info.Name
					_ = info.Type
					_ = info.Path
				}
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 filesystem info doc snippets, got %d", got)
	}
}
