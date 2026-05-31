package doctest

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	e2b "github.com/superduck-ai/e2b-go-sdk"
)

func TestDocsQuickstartInstallCustomPackagesDocumentExists(t *testing.T) {
	if _, err := os.Stat("docs/quickstart/install-custom-packages.mdx"); err != nil {
		t.Fatalf("quickstart install custom packages doc is missing: %v", err)
	}
}

// This test keeps docs/quickstart/install-custom-packages.mdx aligned with the
// exported Go SDK package-install flows. The closures are compile-only
// examples and are intentionally never executed.
func TestDocsQuickstartInstallCustomPackagesExamplesCompile(t *testing.T) {
	snippets := []struct {
		name string
		fn   func()
	}{
		{
			name: "template-definition",
			fn: func() {
				template := e2b.Template(nil).
					FromBaseImage().
					PipInstall([]string{"cowsay"}, nil).
					NpmInstall([]string{"cowsay"}, nil)

				_ = template
			},
		},
		{
			name: "build-script",
			fn: func() {
				_ = godotenv.Load()

				template := e2b.Template(nil).
					FromBaseImage().
					PipInstall([]string{"cowsay"}, nil).
					NpmInstall([]string{"cowsay"}, nil)

				buildInfo, err := e2b.Build(context.Background(), template, "custom-packages", &e2b.BuildOptions{
					BasicBuildOptions: e2b.BasicBuildOptions{
						CpuCount:    2,
						MemoryMB:    2048,
						OnBuildLogs: e2b.DefaultBuildLogger(),
					},
				})

				_ = buildInfo
				_ = err
			},
		},
		{
			name: "use-custom-template",
			fn: func() {
				ctx := context.Background()

				sandbox, err := e2b.Create(ctx, "custom-packages", nil)
				if sandbox != nil {
					defer sandbox.Kill(context.Background(), nil)
				}

				_ = sandbox
				_ = err
			},
		},
		{
			name: "runtime-pip-install",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				installExec, installErr := sandbox.Commands.Run(ctx, "pip install cowsay", nil)
				verifyExec, verifyErr := sandbox.Commands.Run(ctx, `python3 -c "import cowsay; print(cowsay.get_output_string('cow', 'Hello, world!'))"`, nil)
				verifyResult := verifyExec.(*e2b.CommandResult)

				_ = installExec
				_ = verifyResult.Stdout
				_ = installErr
				_ = verifyErr
			},
		},
		{
			name: "runtime-npm-install",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				installExec, installErr := sandbox.Commands.Run(ctx, "npm install cowsay", nil)
				verifyExec, verifyErr := sandbox.Commands.Run(ctx, `node -e "const cowsay = require('cowsay'); console.log(cowsay.say({ text: 'Hello, world!' }))"`, nil)
				verifyResult := verifyExec.(*e2b.CommandResult)

				_ = installExec
				_ = verifyResult.Stdout
				_ = installErr
				_ = verifyErr
			},
		},
		{
			name: "runtime-apt-install",
			fn: func() {
				ctx := context.Background()
				sandbox, err := e2b.Create(ctx, "", nil)
				if err != nil {
					return
				}
				defer sandbox.Kill(context.Background(), nil)

				execution, runErr := sandbox.Commands.Run(ctx, "apt-get update && apt-get install -y curl git", &e2b.CommandStartOpts{
					User: "root",
				})
				result := execution.(*e2b.CommandResult)

				_ = result.ExitCode
				_ = runErr
			},
		},
	}

	if got := len(snippets); got != 6 {
		t.Fatalf("expected 6 quickstart install custom packages doc snippets, got %d", got)
	}
}
