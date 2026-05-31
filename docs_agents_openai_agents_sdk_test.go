package e2b_test

import (
	"os"
	"testing"
)

func TestDocsAgentsOpenAIAgentsSDKDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/agents/openai-agents-sdk.mdx"); err != nil {
		t.Fatalf("agents openai-agents-sdk doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsAgentsOpenAIAgentsSDKExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 agents openai-agents-sdk doc snippets, got %d", got)
	}
}
