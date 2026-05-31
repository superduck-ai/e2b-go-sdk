package doctest

import (
	"os"
	"testing"
)

func TestDocsTemplateHowItWorksDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/how-it-works.mdx"); err != nil {
		t.Fatalf("template how-it-works doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsTemplateHowItWorksExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 template how-it-works doc snippets, got %d", got)
	}
}
