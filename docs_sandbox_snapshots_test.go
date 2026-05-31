package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxSnapshotsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/snapshots.mdx"); err != nil {
		t.Fatalf("sandbox snapshots doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/snapshots.mdx aligned with the exported Go SDK
// snapshots surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxSnapshotsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create-snapshot",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				snapshot, snapshotErr := sandbox.CreateSnapshot(ctx, &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})

				_ = snapshot.SnapshotID
				_ = snapshot.Names
				_ = snapshotErr
			},
		},
		{
			name: "create-snapshot-by-id",
			fn: func() {
				ctx := context.Background()

				snapshot, err := e2b.CreateSnapshot(ctx, "sbx_123", &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})

				_ = snapshot
				_ = err
			},
		},
		{
			name: "restore-from-snapshot",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				snapshot, snapshotErr := sandbox.CreateSnapshot(ctx, nil)
				if snapshotErr != nil {
					return
				}
				restored, restoreErr := e2b.Create(ctx, snapshot.SnapshotID, nil)
				if restored != nil {
					defer restored.Kill(context.Background(), nil)
				}

				_ = restored
				_ = restoreErr
			},
		},
		{
			name: "list-snapshots",
			fn: func() {
				ctx := context.Background()
				paginator := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					Limit: 50,
				})

				var snapshots []e2b.SnapshotInfo
				for paginator.HasNext {
					items, err := paginator.NextItems()
					snapshots = append(snapshots, items...)
					_ = err
				}

				filtered, filteredErr := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: "sbx_123",
					Limit:     20,
				}).NextItems()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}
				instanceSnapshots, instanceErr := sandbox.ListSnapshots(nil).NextItems()

				_ = snapshots
				_ = filtered
				_ = instanceSnapshots
				_ = filteredErr
				_ = instanceErr
			},
		},
		{
			name: "list-snapshots-by-sandbox",
			fn: func() {
				paginator := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: "sbx_123",
					Limit:     20,
				})

				snapshots, err := paginator.NextItems()

				_ = snapshots
				_ = err
			},
		},
		{
			name: "delete-snapshot",
			fn: func() {
				ctx := context.Background()

				deleted, err := e2b.DeleteSnapshot(ctx, "snap_123", nil)

				_ = deleted
				_ = err
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 sandbox snapshots doc snippets, got %d", got)
	}
}
