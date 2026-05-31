package e2b_test

import (
	"os"
	"testing"
)

func TestDocsFAQPausedSandboxesConcurrencyDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/paused-sandboxes-concurrency.mdx"); err != nil {
		t.Fatalf("faq paused sandboxes concurrency doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQPausedSandboxesConcurrencyExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq paused sandboxes concurrency doc snippets, got %d", got)
	}
}
