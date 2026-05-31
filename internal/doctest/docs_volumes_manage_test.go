package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsVolumesManageDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/volumes/manage.mdx"); err != nil {
		t.Fatalf("volumes manage doc is missing: %v", err)
	}
}

// This test keeps docs/volumes/manage.mdx aligned with the exported Go SDK
// volume control-plane surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsVolumesManageExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-volume",
			fn: func() {
				ctx := context.Background()

				volume, err := e2b.CreateVolume(ctx, "my-volume", nil)

				_ = volume.VolumeID
				_ = volume.Name
				_ = volume.Token
				_ = err
			},
		},
		{
			name: "connect-volume",
			fn: func() {
				ctx := context.Background()

				volume, err := e2b.ConnectVolume(ctx, "vol-123", nil)

				_ = volume.VolumeID
				_ = volume.Name
				_ = err
			},
		},
		{
			name: "list-volumes",
			fn: func() {
				ctx := context.Background()

				volumes, err := e2b.ListVolumes(ctx, nil)

				_ = volumes
				_ = err
			},
		},
		{
			name: "get-volume-info",
			fn: func() {
				ctx := context.Background()

				info, err := e2b.GetVolumeInfo(ctx, "vol-123", nil)

				_ = info.VolumeID
				_ = info.Name
				_ = info.Token
				_ = err
			},
		},
		{
			name: "destroy-volume",
			fn: func() {
				ctx := context.Background()

				destroyed, err := e2b.DestroyVolume(ctx, "vol-123", nil)

				_ = destroyed
				_ = err
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 volumes manage doc snippets, got %d", got)
	}
}
