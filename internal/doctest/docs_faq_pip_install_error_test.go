package doctest

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFAQPipInstallErrorDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/faq/pip-install-error.mdx"); err != nil {
		t.Fatalf("faq pip install error doc is missing: %v", err)
	}
}

// This test keeps docs/faq/pip-install-error.mdx aligned with the exported Go
// template builder surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsFAQPipInstallErrorExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "redirect-temp-dir",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("TMPDIR=/var/tmp pip install --no-cache-dir torch sentence-transformers")

				_ = template
			},
		},
		{
			name: "cpu-only-pytorch",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("pip install --no-cache-dir torch --index-url https://download.pytorch.org/whl/cpu").
					RunCmd(`echo "torch" > /tmp/constraints.txt && pip install --no-cache-dir -c /tmp/constraints.txt sentence-transformers`)

				_ = template
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 faq pip install error doc snippets, got %d", got)
	}
}
