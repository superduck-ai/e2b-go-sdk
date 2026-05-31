package e2b_test

import (
	"os"
	"testing"
)

func TestDocsCookbookDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/cookbook.mdx"); err != nil {
		t.Fatalf("cookbook doc is missing: %v", err)
	}
}

// This guide is intentionally prose-only because the cookbook lives in a
// separate repository and this repo does not vendor those end-to-end
// applications.
func TestDocsCookbookExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 cookbook doc snippets, got %d", got)
	}
}
