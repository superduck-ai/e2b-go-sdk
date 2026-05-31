package e2b_test

import (
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateReadyCmdReferenceDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sdk-reference/go-sdk/template-readycmd.mdx"); err != nil {
		t.Fatalf("template readycmd reference doc is missing: %v", err)
	}
}

// This test keeps docs/sdk-reference/go-sdk/template-readycmd.mdx aligned with
// the exported Go SDK ReadyCmd surface. The closures are compile-only examples
// and are intentionally never executed.
func TestDocsTemplateReadyCmdReferenceExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "struct-literal-and-getcmd",
			fn: func() {
				cmd := &e2b.ReadyCmd{
					Command: "curl -fsS http://127.0.0.1:8000/health",
				}

				value := cmd.GetCmd()
				_ = value
			},
		},
		{
			name: "helper-functions",
			fn: func() {
				port := e2b.WaitForPort(8000)
				url := e2b.WaitForURL("http://localhost:3000/health")
				url201 := e2b.WaitForURL("http://localhost:3000/health", 201)
				process := e2b.WaitForProcess("nginx")
				file := e2b.WaitForFile("/tmp/ready")
				timeout := e2b.WaitForTimeout(500)

				_ = port.GetCmd()
				_ = url.GetCmd()
				_ = url201.GetCmd()
				_ = process.GetCmd()
				_ = file.GetCmd()
				_ = timeout.GetCmd()
			},
		},
		{
			name: "template-integration",
			fn: func() {
				portReady := e2b.WaitForPort(8000)

				template := e2b.Template(nil).
					FromPythonImage("3.12").
					SetStartCmd("python -m http.server 8000", portReady).
					SetReadyCmd("curl -fsS http://127.0.0.1:8000/")

				backgroundBuild := e2b.Template(nil).
					FromUbuntuImage("22.04").
					SetStartCmd(`echo "Hello"`, e2b.WaitForTimeout(10_000))

				devcontainer := e2b.Template(nil).
					FromTemplate("devcontainer").
					BetaSetDevContainerStart("/workspace")

				_ = template
				_ = backgroundBuild
				_ = devcontainer
			},
		},
		{
			name: "specialized-template-flows",
			fn: func() {
				backgroundBuild := e2b.Template(nil).
					FromUbuntuImage("22.04").
					SetStartCmd(`echo "Hello"`, e2b.WaitForTimeout(10_000))

				devcontainer := e2b.Template(nil).
					FromTemplate("devcontainer").
					BetaSetDevContainerStart("/workspace")

				_ = backgroundBuild
				_ = devcontainer
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 template readycmd doc snippets, got %d", got)
	}
}
