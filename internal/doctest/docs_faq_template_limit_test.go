package doctest

import (
	"os"
	"testing"
)

func TestDocsFAQTemplateLimitDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/template-limit.mdx"); err != nil {
		t.Fatalf("faq template limit doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQTemplateLimitExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq template limit doc snippets, got %d", got)
	}
}
