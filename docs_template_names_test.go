package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateNamesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/names.mdx"); err != nil {
		t.Fatalf("template names doc is missing: %v", err)
	}
}

// This test keeps docs/template/names.mdx aligned with the exported Go SDK
// naming surface. The closures are compile-only examples and are intentionally
// never executed.
func TestDocsTemplateNamesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-and-create-by-name",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromPythonImage("3.12")

				buildInfo, buildErr := e2b.Build(ctx, template, "my-python-env", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 2,
						MemoryMB: 2048,
					},
				})
				sandbox, sandboxErr := e2b.Create(ctx, "my-python-env", nil)
				legacyInfo, legacyErr := e2b.BuildInBackground(ctx, template, "", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						Alias: "legacy-name",
					},
				})

				if buildInfo != nil {
					_ = buildInfo.Name
					_ = buildInfo.Alias
					_ = buildInfo.TemplateID
					_ = buildInfo.BuildID
				}
				_ = sandbox
				_ = legacyInfo
				_ = buildErr
				_ = sandboxErr
				_ = legacyErr
			},
		},
		{
			name: "team-local-and-namespaced-references",
			fn: func() {
				ctx := context.Background()

				teamLocal, teamLocalErr := e2b.Create(ctx, "my-app", nil)
				publicTemplate, publicErr := e2b.Create(ctx, "acme/my-app", nil)

				_ = teamLocal
				_ = publicTemplate
				_ = teamLocalErr
				_ = publicErr
			},
		},
		{
			name: "environment-names",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				dev, devErr := e2b.Build(ctx, template, "myapp-dev", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 1,
						MemoryMB: 1024,
					},
				})
				prod, prodErr := e2b.Build(ctx, template, "myapp-prod", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 4,
						MemoryMB: 4096,
					},
				})

				_ = dev
				_ = prod
				_ = devErr
				_ = prodErr
			},
		},
		{
			name: "variant-names",
			fn: func() {
				ctx := context.Background()
				template := e2b.Template(nil).FromBaseImage()

				small, smallErr := e2b.Build(ctx, template, "myapp-small", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 1,
						MemoryMB: 512,
					},
				})
				large, largeErr := e2b.Build(ctx, template, "myapp-large", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount: 8,
						MemoryMB: 8192,
					},
				})

				_ = small
				_ = large
				_ = smallErr
				_ = largeErr
			},
		},
		{
			name: "name-availability",
			fn: func() {
				ctx := context.Background()

				exists, existsErr := e2b.Exists(ctx, "my-template", nil)
				legacyExists, legacyErr := e2b.AliasExists(ctx, "my-template", nil)

				_ = exists
				_ = legacyExists
				_ = existsErr
				_ = legacyErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 template names doc snippets, got %d", got)
	}
}
