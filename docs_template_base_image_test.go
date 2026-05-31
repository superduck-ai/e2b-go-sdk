package e2b_test

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateBaseImageDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/base-image.mdx"); err != nil {
		t.Fatalf("template base-image doc is missing: %v", err)
	}
}

// This test keeps docs/template/base-image.mdx aligned with the exported Go
// SDK base-image surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsTemplateBaseImageExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-options",
			fn: func() {
				template := e2b.Template(&e2b.TemplateOptions{
					FileContextPath:    ".",
					FileIgnorePatterns: []string{".git", "node_modules"},
				})

				_ = template
			},
		},
		{
			name: "predefined-base-images",
			fn: func() {
				defaultBase := e2b.Template(nil)
				explicitBase := e2b.Template(nil).FromBaseImage()
				ubuntu := e2b.Template(nil).FromUbuntuImage("22.04")
				debian := e2b.Template(nil).FromDebianImage("stable-slim")
				python := e2b.Template(nil).FromPythonImage("3.13")
				node := e2b.Template(nil).FromNodeImage("lts")
				bun := e2b.Template(nil).FromBunImage("1.3")

				_, _, _, _, _, _, _ = defaultBase, explicitBase, ubuntu, debian, python, node, bun
			},
		},
		{
			name: "custom-images-and-templates",
			fn: func() {
				fromImage := e2b.Template(nil).FromImage("custom-image:latest")

				fromPrivate := e2b.Template(nil).FromImage("ghcr.io/acme/private:latest", &e2b.RegistryCredentials{
					Username: "user",
					Password: "token",
				})

				fromAWS := e2b.Template(nil).FromAWSRegistry(
					"123456789.dkr.ecr.us-west-2.amazonaws.com/app:latest",
					&e2b.AWSRegistryCredentials{
						AccessKeyID:     "AKIA...",
						SecretAccessKey: "...",
						Region:          "us-west-2",
					},
				)

				fromGCP := e2b.Template(&e2b.TemplateOptions{FileContextPath: "."}).FromGCPRegistry(
					"gcr.io/myproject/app:latest",
					&e2b.GCPRegistryCredentials{ServiceAccountJSON: "service-account.json"},
				)

				fromTemplate := e2b.Template(nil).FromTemplate("my-template")
				fromNamespacedTemplate := e2b.Template(nil).FromTemplate("acme/other-template")

				_, _, _, _, _, _ = fromImage, fromPrivate, fromAWS, fromGCP, fromTemplate, fromNamespacedTemplate
			},
		},
		{
			name: "from-dockerfile",
			fn: func() {
				inline := e2b.Template(nil).FromDockerfile(`FROM ubuntu:22.04
RUN apt-get update && apt-get install -y curl
WORKDIR /app
COPY . .
ENV NODE_ENV=production
ENV PORT=3000
USER appuser`)

				fromPath := e2b.Template(&e2b.TemplateOptions{
					FileContextPath: ".",
				}).FromDockerfile("Dockerfile")

				inline.SetStartCmd("npm start", e2b.WaitForTimeout(5_000))

				_ = inline
				_ = fromPath
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template base-image doc snippets, got %d", got)
	}
}
