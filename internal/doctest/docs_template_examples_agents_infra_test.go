package doctest

import (
	"context"
	"os"
	"testing"

	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsTemplateExamplesClaudeCodeDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/claude-code.mdx"); err != nil {
		t.Fatalf("template example claude-code doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/claude-code.mdx aligned with the
// exported Go SDK template, build, sandbox, and command surface. The closures
// are compile-only examples and are intentionally never executed.
func TestDocsTemplateExamplesClaudeCodeExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage("24").
					AptInstall([]string{"curl", "git", "ripgrep"}, nil).
					RunCmd("npm install -g @anthropic-ai/claude-code@latest", &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromNodeImage("24").
					AptInstall([]string{"curl", "git", "ripgrep"}, nil).
					RunCmd("npm install -g @anthropic-ai/claude-code@latest", &struct{ User string }{User: "root"})

				buildInfo, buildErr := e2b.Build(context.Background(), template, "claude-code", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    1,
						MemoryMB:    1024,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "create-sandbox-and-run",
			fn: func() {
				ctx := context.Background()
				timeoutMs := 0

				sandbox, err := e2b.Create(ctx, "claude-code", &e2b.SandboxOpts{
					Envs: map[string]string{
						"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
					},
				})
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(
					ctx,
					`claude --dangerously-skip-permissions -p "Create a hello world index.html"`,
					&e2b.CommandStartOpts{TimeoutMs: &timeoutMs},
				)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template example claude-code doc snippets, got %d", got)
	}
}

func TestDocsTemplateExamplesDesktopDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/desktop.mdx"); err != nil {
		t.Fatalf("template example desktop doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/desktop.mdx aligned with the exported
// Go SDK template, build, sandbox, and host surface. The closures are
// compile-only examples and are intentionally never executed.
func TestDocsTemplateExamplesDesktopExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					RunCmd([]string{
						"yes | unminimize",
						"apt-get update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y xvfb xfce4 xfce4-goodies x11vnc curl git wget net-tools xdotool scrot",
						"git clone --branch e2b-desktop https://github.com/e2b-dev/noVNC.git /opt/noVNC",
						"ln -s /opt/noVNC/vnc.html /opt/noVNC/index.html",
						"git clone --branch v0.12.0 https://github.com/novnc/websockify /opt/noVNC/utils/websockify",
						"apt-get clean",
						"rm -rf /var/lib/apt/lists/*",
					}, &struct{ User string }{User: "root"}).
					Copy("start_command.sh", "/start_command.sh", nil).
					RunCmd("chmod +x /start_command.sh", &struct{ User string }{User: "root"}).
					SetStartCmd("/start_command.sh", e2b.WaitForPort(6080))

				_ = template
			},
		},
		{
			name: "build-template",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("22.04").
					RunCmd([]string{
						"yes | unminimize",
						"apt-get update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y xvfb xfce4 xfce4-goodies x11vnc curl git wget net-tools xdotool scrot",
						"git clone --branch e2b-desktop https://github.com/e2b-dev/noVNC.git /opt/noVNC",
						"ln -s /opt/noVNC/vnc.html /opt/noVNC/index.html",
						"git clone --branch v0.12.0 https://github.com/novnc/websockify /opt/noVNC/utils/websockify",
						"apt-get clean",
						"rm -rf /var/lib/apt/lists/*",
					}, &struct{ User string }{User: "root"}).
					Copy("start_command.sh", "/start_command.sh", nil).
					RunCmd("chmod +x /start_command.sh", &struct{ User string }{User: "root"}).
					SetStartCmd("/start_command.sh", e2b.WaitForPort(6080))

				buildInfo, buildErr := e2b.Build(context.Background(), template, "desktop", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    8,
						MemoryMB:    8192,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "create-sandbox-and-open",
			fn: func() {
				sandbox, err := e2b.Create(context.Background(), "desktop", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				url := "https://" + sandbox.GetHost(6080)

				_ = url
			},
		},
	}

	if got := len(snippets); got != 3 {
		t.Fatalf("expected 3 template example desktop doc snippets, got %d", got)
	}
}

func TestDocsTemplateExamplesDockerDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/template/examples/docker.mdx"); err != nil {
		t.Fatalf("template example docker doc is missing: %v", err)
	}
}

// This test keeps docs/template/examples/docker.mdx aligned with the exported
// Go SDK template, build, sandbox, command, and filesystem surface. The
// closures are compile-only examples and are intentionally never executed.
func TestDocsTemplateExamplesDockerExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "docker-template",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("24.04").
					RunCmd("curl -fsSL https://get.docker.com | sh", &struct{ User string }{User: "root"}).
					RunCmd("usermod -aG docker user", &struct{ User string }{User: "root"}).
					RunCmd("docker run --rm hello-world", &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "docker-build",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("24.04").
					RunCmd("curl -fsSL https://get.docker.com | sh", &struct{ User string }{User: "root"}).
					RunCmd("usermod -aG docker user", &struct{ User string }{User: "root"}).
					RunCmd("docker run --rm hello-world", &struct{ User string }{User: "root"})

				buildInfo, buildErr := e2b.Build(context.Background(), template, "docker", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = buildErr
			},
		},
		{
			name: "docker-run",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "docker", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, `docker run --rm alpine echo "Hello from Alpine!"`, nil)
				result := execution.(*e2b.CommandResult)

				_ = result.Stdout
				_ = runErr
			},
		},
		{
			name: "compose-template",
			fn: func() {
				template := e2b.Template(nil).
					FromUbuntuImage("24.04").
					RunCmd([]string{
						"set -euxo pipefail",
						"apt-get update",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y docker.io",
						"usermod -aG docker user",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y docker-compose-plugin || true",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y docker-compose-v2 || true",
						"DEBIAN_FRONTEND=noninteractive apt-get install -y docker-compose || true",
						"docker compose version || docker-compose --version",
					}, &struct{ User string }{User: "root"})

				_ = template
			},
		},
		{
			name: "compose-run",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "docker-compose", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				writeInfo, writeErr := sandbox.Files.Write(ctx, "/tmp/docker-compose-test/compose.yaml", `services:
  hello:
    image: busybox:1.36
    command: ["sh", "-lc", "echo docker-compose-ok"]
`, nil)
				execution, runErr := sandbox.Commands.Run(ctx, `
set -euxo pipefail
cd /tmp/docker-compose-test

if docker compose version >/dev/null 2>&1; then
  docker compose up --abort-on-container-exit --remove-orphans
  docker compose down --remove-orphans -v
elif docker-compose --version >/dev/null 2>&1; then
  docker-compose up --abort-on-container-exit --remove-orphans
  docker-compose down --remove-orphans -v
else
  echo "No compose command available"
  exit 127
fi
`, nil)
				result := execution.(*e2b.CommandResult)

				_ = writeInfo
				_ = result.Stdout
				_ = writeErr
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 5 {
		t.Fatalf("expected 5 template example docker doc snippets, got %d", got)
	}
}
