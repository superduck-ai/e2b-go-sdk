package e2b_test

import (
	"context"
	"log"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsBillingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/billing.mdx"); err != nil {
		t.Fatalf("billing doc is missing: %v", err)
	}
}

// This test keeps docs/billing.mdx aligned with the exported Go SDK build
// sizing surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsBillingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "customize-build-resources",
			fn: func() {
				template := e2b.Template(nil).FromBaseImage()

				buildInfo, err := e2b.Build(context.Background(), template, "my-template", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    8,
						MemoryMB:    4096,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
				if err != nil {
					log.Print(err)
				}

				_ = buildInfo
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 billing doc snippet, got %d", got)
	}
}
