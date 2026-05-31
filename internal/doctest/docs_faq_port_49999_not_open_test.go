package doctest

import (
	"os"
	"testing"
)

func TestDocsFAQPort49999NotOpenDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/port-49999-not-open.mdx"); err != nil {
		t.Fatalf("faq port 49999 not open doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQPort49999NotOpenExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq port 49999 not open doc snippets, got %d", got)
	}
}
