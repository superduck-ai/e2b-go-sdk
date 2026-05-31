package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem.mdx"); err != nil {
		t.Fatalf("filesystem overview doc is missing: %v", err)
	}
}

// This test keeps docs/filesystem.mdx aligned with the exported Go SDK
// filesystem overview surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsFilesystemExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "filesystem-overview",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}

				entries, listErr := sandbox.Files.List(ctx, "/", nil)
				_ = entries
				_ = listErr
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 filesystem overview doc snippet, got %d", got)
	}
}
