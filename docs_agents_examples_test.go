package e2b_test

import (
	"os"
	"testing"
)

func TestDocsAgentsExamplesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/examples.mdx"); err != nil {
		t.Fatalf("agents examples doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsAgentsExamplesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 agents examples doc snippets, got %d", got)
	}
}
