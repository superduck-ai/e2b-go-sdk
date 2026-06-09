package doctest

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/sandbox.mdx"); err != nil {
		t.Fatalf("sandbox reference doc is missing: %v", err)
	}
}

func TestDocsSandboxReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "create",
			fn: func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()

				timeoutMs := 10 * 60 * 1000

				sandbox, err := e2b.Create(ctx, "", &e2b.SandboxOpts{
					TimeoutMs: &timeoutMs,
					Metadata: map[string]string{
						"service": "docs-example",
					},
				})
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				_ = sandbox.SandboxID
				_ = sandbox.SandboxDomain
				_ = sandbox.Files
				_ = sandbox.Commands
				_ = sandbox.Pty
				_ = sandbox.Git
			},
		},
		{
			name: "connect",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}
				sameSandbox, reconnectErr := sandbox.Connect(ctx, nil)
				assert.NoError(t, reconnectErr, "reconnect")

				_ = sandbox
				_ = sameSandbox
			},
		},
		{
			name: "lifecycle",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				info, infoErr := sandbox.GetInfo(ctx, nil)
				assert.NoError(t, infoErr, "info")
				running, runningErr := sandbox.IsRunning(ctx, nil)
				assert.NoError(t, runningErr, "running")

				timeoutMs := int((30 * time.Minute) / time.Millisecond)
				assert.NoError(t, sandbox.SetTimeout(ctx, timeoutMs, nil), "set timeout")

				start := time.Now().Add(-5 * time.Minute)
				end := time.Now()
				metrics, metricsErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})
				assert.NoError(t, metricsErr, "metrics")

				allowInternetAccess := false
				assert.NoError(t, sandbox.UpdateNetwork(ctx, e2b.SandboxNetworkUpdate{
					AllowInternetAccess: &allowInternetAccess,
				}, nil), "update network")

				paused, pauseErr := sandbox.Pause(ctx, nil)
				assert.NoError(t, pauseErr, "pause")
				assert.NoError(t, sandbox.Kill(ctx, nil), "kill")

				_ = info
				_ = running
				_ = paused
				_ = metrics
			},
		},
		{
			name: "list",
			fn: func(t *testing.T) {
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
				if !assert.NoError(t, err, "next items") {
					return
				}
				itemsWithContext, errWithContext := paginator.NextItemsContext(context.Background())
				assert.NoError(t, errWithContext, "next items with context")

				_ = items
				_ = itemsWithContext
			},
		},
		{
			name: "snapshots",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				snapshot, snapshotErr := sandbox.CreateSnapshot(ctx, &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})
				if !assert.NoError(t, snapshotErr, "create snapshot") {
					return
				}
				restored, restoredErr := e2b.Create(ctx, snapshot.SnapshotID, nil)
				assert.NoError(t, restoredErr, "restore")
				if restored != nil {
					defer restored.Kill(context.Background(), nil)
				}

				instanceSnapshots, instanceSnapshotsErr := sandbox.ListSnapshots(nil).NextItems()
				assert.NoError(t, instanceSnapshotsErr, "list instance snapshots")
				staticSnapshots, staticSnapshotsErr := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: sandbox.SandboxID,
					Limit:     20,
				}).NextItems()
				assert.NoError(t, staticSnapshotsErr, "list static snapshots")

				deleted, deleteErr := e2b.DeleteSnapshot(ctx, snapshot.SnapshotID, nil)
				assert.NoError(t, deleteErr, "delete snapshot")

				_ = snapshot
				_ = restored
				_ = instanceSnapshots
				_ = staticSnapshots
				_ = deleted
			},
		},
		{
			name: "host-and-file-urls",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()
				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if !assert.NoError(t, err, "failed to connect") {
					return
				}

				serviceURL := "https://" + sandbox.GetHost(3000)
				req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
				assert.NoError(t, reqErr, "build request")
				if req != nil && sandbox.TrafficAccessToken != "" {
					req.Header.Set("e2b-traffic-access-token", sandbox.TrafficAccessToken)
				}

				downloadURL, downloadErr := sandbox.DownloadUrl("/tmp/result.json", nil)
				assert.NoError(t, downloadErr, "download URL")
				uploadURL, uploadErr := sandbox.UploadUrl("/tmp/input.json", nil)
				assert.NoError(t, uploadErr, "upload URL")
				mcpURL := sandbox.GetMcpUrl()
				mcpToken, mcpErr := sandbox.GetMcpToken()
				assert.NoError(t, mcpErr, "mcp token")

				_ = req
				_ = downloadURL
				_ = uploadURL
				_ = mcpURL
				_ = mcpToken
			},
		},
		{
			name: "package-level-helpers",
			fn: func(t *testing.T) {
				t.Skip("requires an existing sandbox ID (sbx_123)")

				ctx := context.Background()

				info, infoErr := e2b.GetInfo(ctx, "sbx_123", nil)
				assert.NoError(t, infoErr, "info")
				fullInfo, fullInfoErr := e2b.GetFullInfo(ctx, "sbx_123", nil)
				assert.NoError(t, fullInfoErr, "full info")

				start := time.Now().Add(-5 * time.Minute)
				end := time.Now()
				metrics, metricsErr := e2b.GetMetrics(ctx, "sbx_123", &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})
				assert.NoError(t, metricsErr, "metrics")

				paused, pauseErr := e2b.Pause(ctx, "sbx_123", nil)
				assert.NoError(t, pauseErr, "pause")
				assert.NoError(t, e2b.SetTimeout(ctx, "sbx_123", 10*60*1000, nil), "set timeout")

				allowInternetAccess := false
				assert.NoError(t, e2b.UpdateNetwork(ctx, "sbx_123", e2b.SandboxNetworkUpdate{
					AllowInternetAccess: &allowInternetAccess,
				}, nil), "update network")

				killed, killErr := e2b.Kill(ctx, "sbx_123", nil)
				assert.NoError(t, killErr, "kill")
				snapshot, snapshotErr := e2b.CreateSnapshot(ctx, "sbx_123", &e2b.CreateSnapshotOpts{
					Name: "docs-snapshot",
				})
				assert.NoError(t, snapshotErr, "create snapshot")
				snapshots, snapshotsErr := e2b.ListSnapshots(&e2b.SnapshotListOpts{
					SandboxID: "sbx_123",
					Limit:     20,
				}).NextItems()
				assert.NoError(t, snapshotsErr, "list snapshots")

				deleted, deleteErr := e2b.DeleteSnapshot(ctx, "snap_123", nil)
				assert.NoError(t, deleteErr, "delete snapshot")

				_ = info
				_ = fullInfo
				_ = metrics
				_ = paused
				_ = killed
				_ = snapshot
				_ = snapshots
				_ = deleted
			},
		},
	}

	if got := len(snippets); got != 7 {
		t.Fatalf("expected 7 sandbox doc snippets, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
