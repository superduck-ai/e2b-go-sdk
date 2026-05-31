package e2b_test

import (
	"context"
	"log"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsSandboxIpTunnelingDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/sandbox/ip-tunneling.mdx"); err != nil {
		t.Fatalf("sandbox ip tunneling doc is missing: %v", err)
	}
}

// This test keeps docs/sandbox/ip-tunneling.mdx aligned with the exported Go
// SDK template and sandbox workflow for proxy tunneling. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsSandboxIpTunnelingExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "build-local-proxy-template",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					RunCmd([]string{
						"wget https://github.com/shadowsocks/shadowsocks-rust/releases/latest/download/shadowsocks-v1.24.0.x86_64-unknown-linux-gnu.tar.xz",
						"tar -xf shadowsocks-*.tar.xz",
						"sudo mv sslocal /usr/local/bin/",
					}).
					Copy("config.json", "/root/config.json", nil).
					SetStartCmd(
						"sudo sslocal -c /root/config.json --daemonize",
						e2b.WaitForPort(1080),
					)

				buildInfo, buildErr := e2b.Build(context.Background(), template, "shadowsocks-client-local", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    1,
						MemoryMB:    1024,
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
			name: "use-local-proxy",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "shadowsocks-client-local", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "curl --socks5 127.0.0.1:1080 https://ifconfig.me", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "build-transparent-proxy-template",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					AptInstall("iptables", nil).
					RunCmd([]string{
						"wget https://github.com/shadowsocks/shadowsocks-rust/releases/latest/download/shadowsocks-v1.24.0.x86_64-unknown-linux-gnu.tar.xz",
						"tar -xf shadowsocks-*.tar.xz",
						"sudo mv sslocal /usr/local/bin/",
					}).
					Copy("config.json", "/root/config.json", nil).
					Copy("iptables-rules.sh", "/root/iptables-rules.sh", &struct{ Mode int }{Mode: 0o755}).
					SetStartCmd(
						"sudo sslocal -c /root/config.json --protocol redir -b 0.0.0.0:12345 --daemonize && sudo /root/iptables-rules.sh",
						e2b.WaitForProcess("sslocal"),
					)

				buildInfo, buildErr := e2b.Build(context.Background(), template, "shadowsocks-client-transparent", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    1,
						MemoryMB:    1024,
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
			name: "use-transparent-proxy",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "shadowsocks-client-transparent", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "curl https://ifconfig.me", nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 4 {
		t.Fatalf("expected 4 sandbox ip tunneling doc snippets, got %d", got)
	}
}
