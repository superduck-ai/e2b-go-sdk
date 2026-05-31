package doctest

import (
	"context"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateBuildDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/build.mdx"); err != nil {
		t.Fatalf("template build doc is missing: %v", err)
	}
}

// This test keeps docs/template/build.mdx aligned with the exported Go SDK
// build surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsTemplateBuildExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-and-wait",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 120000

				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd(`echo "hello"`)

				buildInfo, err := e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						SkipCache:   false,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
					ApiKey:           "your-api-key",
					Domain:           "your-domain",
					RequestTimeoutMs: &timeoutMs,
				})

				if buildInfo != nil {
					_ = buildInfo.Name
					_ = buildInfo.Alias
					_ = buildInfo.Tags
					_ = buildInfo.TemplateID
					_ = buildInfo.BuildID
				}
				_ = err
			},
		},
		{
			name: "build-in-background",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				buildInfo, err := e2b.BuildInBackground(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 2,
						MemoryMB: 2048,
					},
				})

				if buildInfo != nil {
					_ = buildInfo.TemplateID
					_ = buildInfo.BuildID
				}
				_ = err
			},
		},
		{
			name: "get-build-status",
			fn: func() {
				ctx := context.Background()

				status, err := e2b.GetBuildStatus(ctx, &e2b.BuildInfo{
					TemplateID: "tmpl_123",
					BuildID:    "bld_123",
				}, &e2b.GetBuildStatusOptions{
					LogsOffset: 0,
				})

				if status != nil {
					_ = status.BuildID
					_ = status.TemplateID
					_ = status.Status
					_ = status.LogEntries
					_ = status.Logs
					if status.Reason != nil {
						_ = status.Reason.Message
						_ = status.Reason.Step
						_ = status.Reason.LogEntries
					}
				}

				buildingStatus := e2b.TemplateBuildStatus("building")
				waitingStatus := e2b.TemplateBuildStatus("waiting")
				readyStatus := e2b.TemplateBuildStatus("ready")
				errorStatus := e2b.TemplateBuildStatus("error")

				_ = err
				_ = buildingStatus
				_ = waitingStatus
				_ = readyStatus
				_ = errorStatus
			},
		},
		{
			name: "poll-build-status",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				buildInfo, err := e2b.BuildInBackground(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 2,
						MemoryMB: 2048,
					},
				})
				if err != nil {
					return
				}

				logsOffset := 0

				for {
					buildStatus, statusErr := e2b.GetBuildStatus(ctx, buildInfo, &e2b.GetBuildStatusOptions{
						LogsOffset: logsOffset,
					})
					if statusErr != nil || buildStatus == nil {
						return
					}

					logsOffset += len(buildStatus.LogEntries)

					for _, entry := range buildStatus.LogEntries {
						_ = entry.String()
					}

					switch buildStatus.Status {
					case e2b.TemplateBuildStatus("ready"):
						return
					case e2b.TemplateBuildStatus("error"):
						return
					}

					time.Sleep(2 * time.Second)
				}
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template build doc snippets, got %d", got)
	}
}
