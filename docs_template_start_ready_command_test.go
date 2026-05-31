package e2b_test

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateStartReadyCommandDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/start-ready-command.mdx"); err != nil {
		t.Fatalf("template start-ready-command doc is missing: %v", err)
	}
}

// This test keeps docs/template/start-ready-command.mdx aligned with the
// exported Go SDK surface. The closures are compile-only examples and are
// intentionally never executed.
func TestDocsTemplateStartReadyCommandExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "runtime-command-after-connect",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Connect(ctx, "sbx_123", nil)
				if err != nil {
					return
				}

				_, _ = sandbox.Commands.Run(ctx, "printenv APP_ENV", nil)
			},
		},
		{
			name: "set-start-cmd",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					SetEnvs(map[string]string{
						"APP_ENV": "production",
					}).
					AptInstall([]string{"curl", "python3"}, nil).
					SetStartCmd("python3 -m http.server 8000", e2b.WaitForPort(8000))

				custom := e2b.Template(nil).
					FromNodeImage().
					SetStartCmd("npm start", "curl -fsS http://localhost:3000/health")

				patterns := e2b.Template(nil)
				patterns.SetStartCmd("npx next --turbo", e2b.WaitForURL("http://localhost:3000"))
				patterns.SetStartCmd("python -m http.server 8000", e2b.WaitForPort(8000))
				patterns.SetStartCmd("/start_command.sh", e2b.WaitForPort(6080))

				_ = template
				_ = custom
				_ = patterns
			},
		},
		{
			name: "set-start-cmd-custom-shell",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage().
					SetStartCmd("npm start", "curl -fsS http://localhost:3000/health")

				_ = template
			},
		},
		{
			name: "set-start-cmd-patterns",
			fn: func() {
				template := e2b.Template(nil)

				template.SetStartCmd("npx next --turbo", e2b.WaitForURL("http://localhost:3000"))
				template.SetStartCmd("python -m http.server 8000", e2b.WaitForPort(8000))
				template.SetStartCmd("/start_command.sh", e2b.WaitForPort(6080))

				_ = template
			},
		},
		{
			name: "set-ready-cmd",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					RunCmd("apt-get install -y nginx && service nginx start", &struct{ User string }{User: "root"}).
					SetReadyCmd(e2b.WaitForPort(80))

				more := e2b.Template(nil)
				more.SetReadyCmd(e2b.WaitForFile("/tmp/ready"))
				more.SetReadyCmd(e2b.WaitForTimeout(10_000))
				more.SetReadyCmd("curl -fsS http://localhost:8080/health")

				_ = template
				_ = more
			},
		},
		{
			name: "set-ready-cmd-more-examples",
			fn: func() {
				template := e2b.Template(nil)

				template.SetReadyCmd(e2b.WaitForFile("/tmp/ready"))
				template.SetReadyCmd(e2b.WaitForTimeout(10_000))
				template.SetReadyCmd("curl -fsS http://localhost:8080/health")

				_ = template
			},
		},
		{
			name: "ready-helper-list",
			fn: func() {
				_ = e2b.WaitForPort(3000)
				_ = e2b.WaitForURL("http://localhost:3000/health")
				_ = e2b.WaitForURL("http://localhost:3000/health", 200)
				_ = e2b.WaitForProcess("node")
				_ = e2b.WaitForFile("/tmp/ready")
				_ = e2b.WaitForTimeout(10_000)
			},
		},
	}

	if got := len(snippets); got != 7 {
		t.Fatalf("expected 7 template start-ready-command doc snippets, got %d", got)
	}
}
