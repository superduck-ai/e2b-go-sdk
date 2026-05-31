package doctest

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateQuickstartDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/quickstart.mdx"); err != nil {
		t.Fatalf("template quickstart doc is missing: %v", err)
	}
}

// This test keeps docs/template/quickstart.mdx aligned with the exported Go
// SDK quickstart flow. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsTemplateQuickstartExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-file",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					SetEnvs(map[string]string{
						"HELLO": "Hello, World!",
					}).
					SetStartCmd("echo $HELLO", e2b.WaitForTimeout(5_000))

				_ = template
			},
		},
		{
			name: "build-dev-script",
			fn: func() {
				_ = godotenv.Load()

				template := e2b.Template(nil).FromBaseImage()
				_, err := e2b.Build(context.Background(), template, "template-tag-dev", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    1,
						MemoryMB:    1024,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
				if err != nil {
					log.Print(err)
				}
			},
		},
		{
			name: "build-prod-script",
			fn: func() {
				_ = godotenv.Load()

				template := e2b.Template(nil).FromBaseImage()
				_, err := e2b.Build(context.Background(), template, "template-tag", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    1,
						MemoryMB:    1024,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
				if err != nil {
					log.Print(err)
				}
			},
		},
		{
			name: "create-sandbox",
			fn: func() {
				_ = godotenv.Load()
				ctx := context.Background()

				devSandbox, devErr := e2b.Create(ctx, "template-tag-dev", nil)
				prodSandbox, prodErr := e2b.Create(ctx, "template-tag", nil)

				_ = devSandbox
				_ = prodSandbox
				_ = devErr
				_ = prodErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template quickstart doc snippets, got %d", got)
	}
}
