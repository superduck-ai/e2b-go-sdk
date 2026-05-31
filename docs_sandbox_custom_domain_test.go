package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxCustomDomainDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/custom-domain.mdx"); err != nil {
		t.Fatalf("sandbox custom domain doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/custom-domain.mdx aligned with the exported Go
// SDK sandbox testing example. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsSandboxCustomDomainExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "test-setup-from-go",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "sudo apt-get update && sudo apt-get install -y nginx && sudo systemctl start nginx", nil)
				result := execution.(*e2b.CommandResult)

				defaultHost := sandbox.GetHost(80)
				customHost := "80-" + sandbox.SandboxID + ".mydomain.com"

				_ = result
				_ = runErr
				_ = defaultHost
				_ = customHost
			},
		},
	}

	if got := len(snippets); got != 1 {
		t.Fatalf("expected 1 sandbox custom domain doc snippet, got %d", got)
	}
}
