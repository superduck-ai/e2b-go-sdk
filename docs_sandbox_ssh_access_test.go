package e2b_test

import (
	"context"
	"log"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxSSHAccessDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/ssh-access.mdx"); err != nil {
		t.Fatalf("sandbox ssh access doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/ssh-access.mdx aligned with the exported Go SDK
// template and sandbox surface for SSH setup. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsSandboxSSHAccessExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-ssh-template",
			fn: func() {
				template := e2b.Template(&e2b.TemplateOptions{
					FileContextPath: ".",
				}).
					FromUbuntuImage("25.04").
					AptInstall([]string{"openssh-server"}, nil).
					MakeDir("/home/user/.ssh", &struct{ Mode int }{Mode: 0o700}).
					Copy("id_ed25519.pub", "/home/user/.ssh/authorized_keys", &struct {
						User string
						Mode int
					}{
						User: "user",
						Mode: 0o600,
					}).
					RunCmd([]string{
						"sudo mkdir -p /run/sshd",
						"curl -fsSL -o /usr/local/bin/websocat https://github.com/vi/websocat/releases/latest/download/websocat.x86_64-unknown-linux-musl",
						"chmod a+x /usr/local/bin/websocat",
					}, &struct{ User string }{User: "root"}).
					SetStartCmd(
						"sudo /usr/sbin/sshd && sudo websocat -b --exit-on-eof ws-l:0.0.0.0:8081 tcp:127.0.0.1:22",
						e2b.WaitForPort(8081),
					)

				buildInfo, buildErr := e2b.Build(context.Background(), template, "ssh-ready", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})
				if buildErr != nil {
					log.Print(buildErr)
				}

				_ = buildInfo
			},
		},
		{
			name: "create-sandbox",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "ssh-ready", nil)

				_ = sandbox
				_ = err
			},
		},
	}

	if got := len(snippets); got != 2 {
		t.Fatalf("expected 2 sandbox ssh access doc snippets, got %d", got)
	}
}
