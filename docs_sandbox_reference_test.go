package e2b_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox.mdx"); err != nil {
		t.Fatalf("sandbox reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/sandbox.mdx aligned with the
// exported Go SDK surface. The closures are intentionally never executed;
// compile success is the contract we care about.
func TestDocsSandboxReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "create",
			fn: func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				timeoutMs := 10 * 60 * 1000

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Metadata: map[string]string{
						"service": "docs-example",
					},
				})
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
				_ = sandbox.SandboxDomain
				_ = sandbox.Files
				_ = sandbox.Commands
				_ = sandbox.Pty
				_ = sandbox.Git
				_ = err
			},
		},
		{
			name: "connect",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				sameSandbox, reconnectErr := sandbox.Connect(ctx, nil)

				_ = sandbox
				_ = sameSandbox
				_ = err
				_ = reconnectErr
			},
		},
		{
			name: "lifecycle",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				info, infoErr := sandbox.GetInfo(ctx, nil)
				running, runningErr := sandbox.IsRunning(ctx, nil)

				timeoutMs := int((30 * time.Minute) / time.Millisecond)
				timeoutErr := sandbox.SetTimeout(ctx, timeoutMs, nil)

				start := time.Now().Add(-5 * time.Minute)
				end := time.Now()
				metrics, metricsErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})

				allowInternetAccess := false
				networkErr := sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
					AllowInternetAccess: &allowInternetAccess,
				}, nil)

				paused, pauseErr := sandbox.Pause(ctx, nil)
				killErr := sandbox.Kill(ctx, nil)

				_ = info
				_ = running
				_ = paused
				_ = metrics
				_ = infoErr
				_ = runningErr
				_ = timeoutErr
				_ = metricsErr
				_ = networkErr
				_ = pauseErr
				_ = killErr
			},
		},
		{
			name: "list",
			fn: func() {
				paginator := e2b.List(&e2b.SandboxListOpts{
					Query: &struct {
						Metadata map[string]string
						State    []e2b.SandboxState
					}{
						Metadata: map[string]string{"service": "docs-example"},
						State:    []e2b.SandboxState{e2b.SandboxState("running")},
					},
					Limit: 20,
				})

				items, err := paginator.NextItems()
				itemsWithContext, errWithContext := paginator.NextItemsContext(context.Background())

				_ = items
				_ = itemsWithContext
				_ = err
				_ = errWithContext
			},
		},
		{
			name: "snapshots",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				snapshot, snapshotErr := sandbox.CreateSnapshot(ctx, &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})
				restored, restoredErr := e2b.Create(ctx, snapshot.SnapshotID, nil)
				defer restored.Kill(context.Background(), nil)

				instanceSnapshots, instanceSnapshotsErr := sandbox.ListSnapshots(nil).NextItems()
				staticSnapshots, staticSnapshotsErr := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: sandbox.SandboxID,
					Limit:     20,
				}).NextItems()

				deleted, deleteErr := e2b.DeleteSnapshot(ctx, snapshot.SnapshotID, nil)

				_ = snapshot
				_ = restored
				_ = instanceSnapshots
				_ = staticSnapshots
				_ = deleted
				_ = snapshotErr
				_ = restoredErr
				_ = instanceSnapshotsErr
				_ = staticSnapshotsErr
				_ = deleteErr
			},
		},
		{
			name: "host-and-file-urls",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				serviceURL := "https://" + sandbox.GetHost(3000)
				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
				if req != nil && sandbox.TrafficAccessToken != "" {
					req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
				}

				downloadURL, downloadErr := sandbox.DownloadUrl("/tmp/result.json", nil)
				uploadURL, uploadErr := sandbox.UploadUrl("/tmp/input.json", nil)
				mcpURL := sandbox.GetMcpUrl()
				mcpToken, mcpErr := sandbox.GetMcpToken()

				_ = req
				_ = reqErr
				_ = downloadURL
				_ = uploadURL
				_ = mcpURL
				_ = mcpToken
				_ = downloadErr
				_ = uploadErr
				_ = mcpErr
			},
		},
		{
			name: "package-level-helpers",
			fn: func() {
				ctx := context.Background()

				info, infoErr := e2b.GetInfo(ctx, "sbx_123", nil)
				fullInfo, fullInfoErr := e2b.GetFullInfo(ctx, "sbx_123", nil)

				start := time.Now().Add(-5 * time.Minute)
				end := time.Now()
				metrics, metricsErr := e2b.GetMetrics(ctx, "sbx_123", &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})

				paused, pauseErr := e2b.Pause(ctx, "sbx_123", nil)
				timeoutErr := e2b.SetTimeout(ctx, "sbx_123", 10*60*1000, nil)

				allowInternetAccess := false
				networkErr := e2b.UpdateNetwork(ctx, "sbx_123", e2b.SandboxNetworkUpdate{
					AllowInternetAccess: &allowInternetAccess,
				}, nil)

				killed, killErr := e2b.Kill(ctx, "sbx_123", nil)
				snapshot, snapshotErr := e2b.CreateSnapshot(ctx, "sbx_123", &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})
				snapshots, snapshotsErr := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: "sbx_123",
					Limit:     20,
				}).NextItems()

				deleted, deleteErr := e2b.DeleteSnapshot(ctx, "snap_123", nil)

				_ = info
				_ = fullInfo
				_ = metrics
				_ = paused
				_ = killed
				_ = snapshot
				_ = snapshots
				_ = deleted
				_ = infoErr
				_ = fullInfoErr
				_ = metricsErr
				_ = pauseErr
				_ = timeoutErr
				_ = networkErr
				_ = killErr
				_ = snapshotErr
				_ = snapshotsErr
				_ = deleteErr
			},
		},
	}

	if got := len(snippets); got != 7 {
		t.Fatalf("expected 7 sandbox doc snippets, got %d", got)
	}
}
