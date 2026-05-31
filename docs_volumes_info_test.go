package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesInfoDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/info.mdx"); err != nil {
		t.Fatalf("volumes info doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/info.mdx aligned with the exported Go SDK
// volume metadata surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesInfoExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "file-metadata",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				_, _ = volume.WriteFile(ctx, "/test_file.txt", "Hello, world!", nil)
				info, infoErr := volume.GetInfo(ctx, "/test_file.txt", nil)

				_ = info.Name
				_ = info.Type
				_ = info.Path
				_ = info.Size
				_ = info.Mode
				_ = info.UID
				_ = info.GID
				_ = info.Atime
				_ = info.Mtime
				_ = info.Ctime
				_ = infoErr
			},
		},
		{
			name: "directory-metadata",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				_, _ = volume.MakeDir(ctx, "/test_dir", nil)
				info, infoErr := volume.GetInfo(ctx, "/test_dir", nil)

				_ = info.Name
				_ = info.Type
				_ = info.Path
				_ = infoErr
			},
		},
		{
			name: "exists",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				exists, existsErr := volume.Exists(ctx, "/test_file.txt", nil)

				_ = exists
				_ = existsErr
			},
		},
		{
			name: "update-metadata",
			fn: func() {
				ctx := context.Background()
				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)
				if err != nil {
					return
				}

				_, _ = volume.WriteFile(ctx, "/test_file.txt", "Hello, world!", nil)

				uid := 1000
				gid := 1000
				mode := 0o600

				updated, updateErr := volume.UpdateMetadata(ctx, "/test_file.txt", &e2b.VolumeMetadataOptions{
					UID:  &uid,
					GID:  &gid,
					Mode: &mode,
				}, nil)

				_ = updated.Name
				_ = updated.Mode
				_ = updated.UID
				_ = updated.GID
				_ = updateErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 volumes info doc snippets, got %d", got)
	}
}
