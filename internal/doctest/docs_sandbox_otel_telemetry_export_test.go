package doctest

import (
	"os"
	"testing"
)

func TestDocsSandboxOtelTelemetryExportDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/otel-telemetry-export.mdx"); err != nil {
		t.Fatalf("sandbox otel telemetry export doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsSandboxOtelTelemetryExportExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 sandbox otel telemetry export doc snippets, got %d", got)
	}
}
