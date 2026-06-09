package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsFilesystemDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/filesystem.mdx"); err != nil {
		t.Fatalf("filesystem overview doc is missing: %v", err)
	}
}

func TestDocsFilesystemExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "filesystem-overview",
			fn: func(t *testing.T) {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if !assert.NoError(t, err, "failed to create sandbox") {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				entries, listErr := sandbox.Files.List(ctx, "/", nil)
				if !assert.NoError(t, listErr, "failed to list /") {
					return
				}
				_ = entries
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 filesystem overview doc snippet, got %d", got)
	}

	for _, snippet := range snippets {
		snippet := snippet
		t.Run(snippet.name, func(t *testing.T) {
			snippet.fn(t)
		})
	}
}
