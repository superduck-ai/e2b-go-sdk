package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemInfoDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem/info.mdx"); err != nil {
		t.Fatalf("filesystem info doc is missing: %v", err)
	}
}

func TestDocsFilesystemInfoExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "file-info",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, writeErr := sandbox.Files.Write(ctx, "test_file.txt", "Hello, world!", nil)
				if !assert.NoError(t, writeErr, "failed to write file") {
					return
				}
				info, infoErr := sandbox.Files.GetInfo(ctx, "test_file.txt", nil)
				if !assert.NoError(t, infoErr, "failed to get file info") {
					return
				}
				if assert.NotNil(t, info, "info should not be nil") {
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
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_, mkErr := sandbox.Files.MakeDir(ctx, "test_dir", nil)
				if !assert.NoError(t, mkErr, "failed to make dir") {
					return
				}
				info, infoErr := sandbox.Files.GetInfo(ctx, "test_dir", nil)
				if !assert.NoError(t, infoErr, "failed to get dir info") {
					return
				}
				if assert.NotNil(t, info, "info should not be nil") {
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

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
