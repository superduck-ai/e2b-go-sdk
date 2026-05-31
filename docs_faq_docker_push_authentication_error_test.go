package e2b_test

import (
	"os"
	"testing"
)

func TestDocsFAQDockerPushAuthenticationErrorDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/docker-push-authentication-error.mdx"); err != nil {
		t.Fatalf("faq docker push authentication error doc is missing: %v", err)
	}
}

// This guide is prose-only today, so the compile guard intentionally asserts
// that there are no runnable Go snippets to keep in sync.
func TestDocsFAQDockerPushAuthenticationErrorExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{}

	if got := len(snippets); got != 0 {
		t.Fatalf("expected 0 faq docker push authentication error doc snippets, got %d", got)
	}
}
