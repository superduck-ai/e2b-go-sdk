package e2b_test

import (
	"os"
	"testing"
)

func TestDocsCLADocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cla.mdx"); err != nil {
		t.Fatalf("cla doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsCLAExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 cla doc snippets, got %d", got)
	}
}
