package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFAQPublicTemplatesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/public-templates.mdx"); err != nil {
		t.Fatalf("faq public templates doc is missing: %v", err)
	}
}

// This test keeps docs/faq/public-templates.mdx aligned with the exported Go
// template-creation surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsFAQPublicTemplatesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "use-public-template",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "your-team-slug/template-name", nil)

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 faq public templates doc snippet, got %d", got)
	}
}
