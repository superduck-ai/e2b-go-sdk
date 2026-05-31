package e2b_test

import (
	"context"
	"os"
	"testing"
	"time"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateLoggerReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/template-logger.mdx"); err != nil {
		t.Fatalf("template logger reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/template-logger.mdx aligned with
// the exported Go SDK logger surface. The closures are compile-only examples
// and are intentionally never executed.
func TestDocsTemplateLoggerReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "log-entry-and-level",
			fn: func() {
				entry := &e2b.LogEntry{
					Timestamp: time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
					Level:     e2b.LogEntryLevel("info"),
					Message:   "Uploaded 'src'",
				}

				debugLevel := e2b.LogEntryLevel("debug")
				infoLevel := e2b.LogEntryLevel("info")
				warnLevel := e2b.LogEntryLevel("warn")
				errorLevel := e2b.LogEntryLevel("error")

				_ = entry.Timestamp
				_ = entry.Level
				_ = entry.Message
				_ = entry.String()
				_ = debugLevel
				_ = infoLevel
				_ = warnLevel
				_ = errorLevel
			},
		},
		{
			name: "start-end-and-default-logger",
			fn: func() {
				start := e2b.NewLogEntryStart("Build started")
				end := e2b.NewLogEntryEnd("Build finished")
				logger := e2b.DefaultBuildLogger()

				logger(&e2b.LogEntry{
					Level:   e2b.LogEntryLevel("info"),
					Message: "Uploaded 'src'\n",
				})

				_ = start.Timestamp
				_ = start.Level
				_ = start.Message
				_ = end.Timestamp
				_ = end.Level
				_ = end.Message
				_ = logger
			},
		},
		{
			name: "build-logger-type",
			fn: func() {
				var logger e2b.BuildLogger = func(entry *e2b.LogEntry) {
					_ = entry
				}

				_ = logger
			},
		},
		{
			name: "build-options-logger",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd(`echo "hello"`)

				logger := e2b.BuildLogger(func(entry *e2b.LogEntry) {
					switch entry.Level {
					case e2b.LogEntryLevel("error"):
						_ = entry.Message
					default:
						_ = entry.String()
					}
				})

				_, _ = e2b.Build(ctx, template, "docs-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						OnBuildLogs: logger,
					},
				})
			},
		},
		{
			name: "default-build-logger",
			fn: func() {
				logger := e2b.DefaultBuildLogger()

				logger(&e2b.LogEntry{
					Level:   e2b.LogEntryLevel("info"),
					Message: "Uploaded 'src'\n",
				})

				_ = logger
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 template logger doc snippets, got %d", got)
	}
}
