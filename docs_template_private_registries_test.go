package e2b_test

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplatePrivateRegistriesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/private-registries.mdx"); err != nil {
		t.Fatalf("template private-registries doc is missing: %v", err)
	}
}

// This test keeps docs/template/private-registries.mdx aligned with the
// exported Go SDK registry-auth surface. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsTemplatePrivateRegistriesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "general-registry",
			fn: func() {
				template := e2b.Template(nil).FromImage("ubuntu:22.04", &e2b.RegistryCredentials{
					Username: "user",
					Password: "pass",
				})

				_ = template
			},
		},
		{
			name: "gcp-from-file",
			fn: func() {
				template := e2b.Template(&e2b.TemplateOptions{
					FileContextPath: ".",
				}).FromGCPRegistry("ubuntu:22.04", &e2b.GCPRegistryCredentials{
					ServiceAccountJSON: "./service_account.json",
				})

				_ = template
			},
		},
		{
			name: "gcp-from-object",
			fn: func() {
				template := e2b.Template(nil).FromGCPRegistry("ubuntu:22.04", &e2b.GCPRegistryCredentials{
					ServiceAccountJSON: map[string]string{
						"project_id":     "123",
						"private_key_id": "456",
					},
				})

				_ = template
			},
		},
		{
			name: "aws-ecr",
			fn: func() {
				template := e2b.Template(nil).FromAWSRegistry("ubuntu:22.04", &e2b.AWSRegistryCredentials{
					AccessKeyID:     "123",
					SecretAccessKey: "456",
					Region:          "us-west-1",
				})

				_ = template
			},
		},
		{
			name: "aws-ecr-session-token",
			fn: func() {
				template := e2b.Template(nil).FromAWSRegistry("ubuntu:22.04", &e2b.AWSRegistryCredentials{
					AccessKeyID:     "123",
					SecretAccessKey: "456",
					SessionToken:    "temporary-session-token",
					Region:          "us-west-1",
				})

				_ = template
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 template private registries doc snippets, got %d", got)
	}
}
