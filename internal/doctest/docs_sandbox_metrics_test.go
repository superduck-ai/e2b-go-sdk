package doctest

import (
	"context"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxMetricsDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/metrics.mdx"); err != nil {
		t.Fatalf("sandbox metrics doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/metrics.mdx aligned with the exported Go SDK
// metrics surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxMetricsExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "connected-sandbox-metrics",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				start := time.Now().Add(-10 * time.Minute)
				end := time.Now()

				metrics, metricsErr := sandbox.GetMetrics(ctx, &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})

				var metric e2b.SandboxMetrics
				if len(metrics) > 0 {
					metric = metrics[0]
				}

				_ = metric.Timestamp
				_ = metric.CpuUsedPct
				_ = metric.CpuCount
				_ = metric.MemUsed
				_ = metric.MemTotal
				_ = metric.MemCache
				_ = metric.DiskUsed
				_ = metric.DiskTotal
				_ = metricsErr
			},
		},
		{
			name: "metrics-by-sandbox-id",
			fn: func() {
				ctx := context.Background()
				start := time.Now().Add(-5 * time.Minute)
				end := time.Now()

				metrics, err := e2b.GetMetrics(ctx, "sbx_123", &e2b.SandboxMetricsOpts{
					Start: &start,
					End:   &end,
				})

				_ = metrics
				_ = err
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 sandbox metrics doc snippets, got %d", got)
	}
}
