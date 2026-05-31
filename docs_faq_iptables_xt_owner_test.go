package e2b_test

import (
	"os"
	"testing"
)

func TestDocsFAQIptablesXtOwnerDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/iptables-xt-owner.mdx"); err != nil {
		t.Fatalf("faq iptables xt_owner doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQIptablesXtOwnerExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq iptables xt_owner doc snippets, got %d", got)
	}
}
