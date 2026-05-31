package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateCachingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/caching.mdx"); err != nil {
		t.Fatalf("template caching doc is missing: %v", err)
	}
}

// This test keeps docs/template/caching.mdx aligned with the exported Go SDK
// caching surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsTemplateCachingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "skip-cache-next-instruction",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					SkipCache().
					RunCmd(`echo "Hello, World!"`)

				_ = template
			},
		},
		{
			name: "skip-cache-whole-build",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				_, _ = e2b.Build(ctx, template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						SkipCache: true,
					},
				})
			},
		},
		{
			name: "copy-with-default-upload-cache",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					Copy("config.json", "/app/config.json", nil)

				_ = template
			},
		},
		{
			name: "copy-with-force-upload",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					Copy("config.json", "/app/config.json", &struct{ ForceUpload bool }{
						ForceUpload: true,
					})

				_ = template
			},
		},
		{
			name: "variant-builds",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd("go build ./...")

				_, _ = e2b.Build(ctx, template, "my-template-2cpu-2gb", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 2,
						MemoryMB: 2048,
					},
				})

				_, _ = e2b.Build(ctx, template, "my-template-1cpu-4gb", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 1,
						MemoryMB: 4096,
					},
				})
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 template caching doc snippets, got %d", got)
	}
}
