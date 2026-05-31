package e2b_test

import (
	"context"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateLoggingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/logging.mdx"); err != nil {
		t.Fatalf("template logging doc is missing: %v", err)
	}
}

// This test keeps docs/template/logging.mdx aligned with the exported Go SDK
// logging surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsTemplateLoggingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "default-logger",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).
					FromPythonImage("3.12").
					RunCmd(`echo "hello"`)

				_, _ = e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
			},
		},
		{
			name: "custom-loggers",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				simple := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					_ = entry.String()
				})

				formatted := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					_ = entry.Timestamp.Format("2006-01-02T15:04:05Z07:00")
					_ = string(entry.Level)
					_ = entry.Message
				})

				filtered := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					switch entry.Level {
					case e2b.LogEntryLevel("error"), e2b.LogEntryLevel("warn"):
						_ = entry.String()
					}
				})

				_, _ = e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: simple,
					},
				})
				_, _ = e2b.BuildInBackground(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: formatted,
					},
				})
				_, _ = e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: filtered,
					},
				})
			},
		},
		{
			name: "custom-logger-formatting",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				logger := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					_ = entry.Timestamp.Format("2006-01-02T15:04:05Z07:00")
					_ = string(entry.Level)
					_ = entry.Message
				})

				_, _ = e2b.BuildInBackground(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: logger,
					},
				})
			},
		},
		{
			name: "custom-logger-filtering",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				logger := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					switch entry.Level {
					case e2b.LogEntryLevel("error"), e2b.LogEntryLevel("warn"):
						_ = entry.String()
					}
				})

				_, _ = e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: logger,
					},
				})
			},
		},
		{
			name: "structured-log-entry",
			fn: func() {
				entry := &e2b.LogEntry{
					Timestamp: time.Now(),
					Level:     e2b.LogEntryLevel("info"),
					Message:   "Uploaded 'src'",
				}

				_ = entry.Timestamp
				_ = entry.Level
				_ = entry.Message
				_ = entry.String()
			},
		},
		{
			name: "start-and-end-helpers",
			fn: func() {
				start := e2b.NewLogEntryStart("Build started")
				end := e2b.NewLogEntryEnd("Build finished")

				_ = start.Timestamp
				_ = start.Level
				_ = start.Message
				_ = end.Timestamp
				_ = end.Level
				_ = end.Message
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 template logging doc snippets, got %d", got)
	}
}
