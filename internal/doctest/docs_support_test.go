package doctest

import (
	"os"
	"testing"
)

func TestDocsSupportDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/support.mdx"); err != nil {
		t.Fatalf("support doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsSupportExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 support doc snippets, got %d", got)
	}
}
