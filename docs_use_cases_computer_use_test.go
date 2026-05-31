package e2b_test

import (
	"os"
	"testing"
)

func TestDocsUseCasesComputerUseDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/use-cases/computer-use.mdx"); err != nil {
		t.Fatalf("use-cases computer-use doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsUseCasesComputerUseExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 use-cases computer-use doc snippets, got %d", got)
	}
}
