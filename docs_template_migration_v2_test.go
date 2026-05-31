package e2b_test

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateMigrationV2DocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/migration-v2.mdx"); err != nil {
		t.Fatalf("template migration-v2 doc is missing: %v", err)
	}
}

// This test keeps docs/template/migration-v2.mdx aligned with the exported Go
// SDK migration-related template entry points. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsTemplateMigrationV2ExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "from-dockerfile-content",
			fn: func() {
				dockerfileContent := `
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y curl
WORKDIR /app
COPY . .
ENV NODE_ENV=production
ENTRYPOINT ["npm", "start"]
`

				template := e2b.Template(nil).FromDockerfile(dockerfileContent)

				_ = template
			},
		},
		{
			name: "from-dockerfile-path",
			fn: func() {
				template := e2b.Template(&e2b.TemplateOptions{
					FileContextPath: ".",
				}).FromDockerfile("Dockerfile")

				_ = template
			},
		},
		{
			name: "from-image",
			fn: func() {
				template := e2b.Template(nil).FromImage("image-tag")

				_ = template
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template migration-v2 doc snippets, got %d", got)
	}
}
