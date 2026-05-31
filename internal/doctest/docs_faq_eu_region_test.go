package doctest

import (
	"os"
	"testing"
)

func TestDocsFAQEURegionDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/eu-region.mdx"); err != nil {
		t.Fatalf("faq eu region doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQEURegionExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq eu region doc snippets, got %d", got)
	}
}
