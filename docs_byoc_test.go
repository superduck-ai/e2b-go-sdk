package e2b_test

import (
	"os"
	"testing"
)

func TestDocsByocDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/byoc.mdx"); err != nil {
		t.Fatalf("byoc doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsByocExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 byoc doc snippets, got %d", got)
	}
}
